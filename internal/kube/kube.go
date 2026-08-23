// Package kube is a small Kubernetes JSON client.
//
// It loads kubeconfig (including exec/OIDC plugins) and speaks the REST API
// with encoding/json. No client-go. Argo Application CRs are the first
// consumer; the same client can GET/PATCH other namespaced resources later.
package kube

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	MergePatch          = "application/merge-patch+json"
	defaultTimeout      = 20 * time.Second
	maxIdleConns        = 32
	maxIdleConnsPerHost = 8
	maxErrBody          = 2048
)

var (
	ErrNoConfig  = errors.New("kubeconfig not found")
	ErrNoContext = errors.New("kube context not found")
)

// REST is a JSON Kubernetes API client for one cluster/context.
type REST struct {
	BaseURL string
	HTTP    *http.Client
	Auth    TokenSource
}

// TokenSource yields a bearer token. Invalidate drops a cached token so the
// next Token call refreshes (exec plugins, 401 retry).
type TokenSource interface {
	Token(ctx context.Context) (string, error)
	Invalidate()
}

// Bearer is a static token.
type Bearer string

func (b Bearer) Token(context.Context) (string, error) {
	return strings.TrimSpace(string(b)), nil
}

func (b Bearer) Invalidate() {}

// fileToken re-reads a projected service-account token on each request.
type fileToken string

func (f fileToken) Token(context.Context) (string, error) {
	b, err := os.ReadFile(string(f))
	if err != nil {
		return "", fmt.Errorf("kube tokenFile: %w", err)
	}
	t := strings.TrimSpace(string(b))
	if t == "" {
		return "", fmt.Errorf("kube tokenFile %s: empty", f)
	}
	return t, nil
}

func (f fileToken) Invalidate() {}

// ResolvePath picks a kubeconfig file. explicit wins, then KUBECONFIG (first
// entry), then ~/.kube/config. Empty means none.
func ResolvePath(explicit string) string {
	if explicit != "" {
		return explicit
	}
	if v := strings.TrimSpace(os.Getenv("KUBECONFIG")); v != "" {
		p, _, _ := strings.Cut(v, string(os.PathListSeparator))
		return strings.TrimSpace(p)
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".kube", "config")
}

// LoadREST builds a client from kubeconfig. Empty path uses ResolvePath("").
// Empty context uses current-context. Does not contact the cluster or exec
// plugins until the first request.
func LoadREST(path, contextName string) (*REST, error) {
	if path == "" {
		path = ResolvePath("")
	}
	if path == "" {
		return nil, ErrNoConfig
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrNoConfig, path)
		}
		return nil, fmt.Errorf("read kubeconfig %s: %w", path, err)
	}
	cfg, err := parseConfig(b)
	if err != nil {
		return nil, fmt.Errorf("parse kubeconfig %s: %w", path, err)
	}
	if contextName == "" {
		contextName = cfg.CurrentContext
	}
	if contextName == "" {
		return nil, fmt.Errorf("%w: no current-context", ErrNoContext)
	}
	ctx, ok := cfg.context(contextName)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrNoContext, contextName)
	}
	cluster, ok := cfg.cluster(ctx.Cluster)
	if !ok {
		return nil, fmt.Errorf("kubeconfig: cluster %q not found", ctx.Cluster)
	}
	user, ok := cfg.user(ctx.User)
	if !ok {
		return nil, fmt.Errorf("kubeconfig: user %q not found", ctx.User)
	}
	httpClient, err := httpClientFor(cluster, user)
	if err != nil {
		return nil, err
	}
	auth, err := tokenSourceFor(user, cluster)
	if err != nil {
		return nil, err
	}
	return &REST{
		BaseURL: strings.TrimRight(cluster.Server, "/"),
		HTTP:    httpClient,
		Auth:    auth,
	}, nil
}

func httpClientFor(cluster cluster, user user) (*http.Client, error) {
	tlsConfig := &tls.Config{}
	if cluster.InsecureSkipTLSVerify {
		tlsConfig.InsecureSkipVerify = true
	}
	caPEM, err := pemFrom(cluster.CertificateAuthorityData, cluster.CertificateAuthority)
	if err != nil {
		return nil, fmt.Errorf("kube cluster CA: %w", err)
	}
	if len(caPEM) > 0 {
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caPEM) {
			return nil, fmt.Errorf("kube cluster CA: no PEM certificates")
		}
		tlsConfig.RootCAs = pool
	}
	certPEM, err := pemFrom(user.ClientCertificateData, user.ClientCertificate)
	if err != nil {
		return nil, fmt.Errorf("kube client cert: %w", err)
	}
	keyPEM, err := pemFrom(user.ClientKeyData, user.ClientKey)
	if err != nil {
		return nil, fmt.Errorf("kube client key: %w", err)
	}
	if len(certPEM) > 0 || len(keyPEM) > 0 {
		cert, err := tls.X509KeyPair(certPEM, keyPEM)
		if err != nil {
			return nil, fmt.Errorf("kube client cert: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}
	if len(tlsConfig.NextProtos) == 0 {
		tlsConfig.NextProtos = []string{"h2", "http/1.1"}
	}
	var tr *http.Transport
	if base, ok := http.DefaultTransport.(*http.Transport); ok {
		tr = base.Clone()
	} else {
		tr = &http.Transport{Proxy: http.ProxyFromEnvironment}
	}
	tr.TLSClientConfig = tlsConfig
	tr.ForceAttemptHTTP2 = true
	tr.MaxIdleConns = maxIdleConns
	tr.MaxIdleConnsPerHost = maxIdleConnsPerHost
	return &http.Client{Timeout: defaultTimeout, Transport: tr}, nil
}

func pemFrom(b64, path string) ([]byte, error) {
	if strings.TrimSpace(b64) != "" {
		raw, err := decodeB64(b64)
		if err != nil {
			return nil, err
		}
		return raw, nil
	}
	if path == "" {
		return nil, nil
	}
	return os.ReadFile(path)
}

func decodeB64(s string) ([]byte, error) {
	s = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == ' ' || r == '\t' {
			return -1
		}
		return r
	}, s)
	return base64.StdEncoding.DecodeString(s)
}

func tokenSourceFor(user user, cluster cluster) (TokenSource, error) {
	if t := strings.TrimSpace(user.Token); t != "" {
		return Bearer(t), nil
	}
	if user.TokenFile != "" {
		return fileToken(user.TokenFile), nil
	}
	if user.Exec != nil && user.Exec.Command != "" {
		return newExecAuth(*user.Exec, cluster), nil
	}
	if user.ClientCertificate != "" || user.ClientCertificateData != "" {
		return Bearer(""), nil
	}
	return nil, fmt.Errorf("kubeconfig: no token, exec plugin, or client cert")
}

// Get JSON-decodes a GET of an absolute API path (for example
// /api/v1/namespaces/ns/pods) into out.
func (c *REST) Get(ctx context.Context, path string, out any) error {
	return c.Do(ctx, http.MethodGet, path, "", nil, out)
}

// Patch JSON-encodes body as a PATCH (usually MergePatch) of an absolute API path.
func (c *REST) Patch(ctx context.Context, path, contentType string, body any) error {
	return c.Do(ctx, http.MethodPatch, path, contentType, body, nil)
}

// Do sends a JSON request. path must start with /.
func (c *REST) Do(ctx context.Context, method, path, contentType string, body, out any) error {
	if c == nil || c.BaseURL == "" {
		return fmt.Errorf("kube client: no server")
	}
	if !strings.HasPrefix(path, "/") {
		return fmt.Errorf("kube path %q must start with /", path)
	}
	var last error
	for attempt := 0; attempt < 2; attempt++ {
		err := c.doOnce(ctx, method, path, contentType, body, out)
		if err == nil {
			return nil
		}
		last = err
		var se *StatusError
		if !errors.As(err, &se) || se.Code != http.StatusUnauthorized || c.Auth == nil {
			return err
		}
		c.Auth.Invalidate()
	}
	return last
}

func (c *REST) doOnce(ctx context.Context, method, path, contentType string, body, out any) error {
	var rdr io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, rdr)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		if contentType == "" {
			contentType = "application/json"
		}
		req.Header.Set("Content-Type", contentType)
	}
	if c.Auth != nil {
		tok, err := c.Auth.Token(ctx)
		if err != nil {
			return err
		}
		if tok != "" {
			req.Header.Set("Authorization", "Bearer "+tok)
		}
	}
	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultTimeout}
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &StatusError{Method: method, Path: path, Code: resp.StatusCode, Status: resp.Status, Body: truncate(raw)}
	}
	if out == nil || resp.StatusCode == http.StatusNoContent || len(raw) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("kube decode %s %s: %w", method, path, err)
	}
	return nil
}

// StatusError is a non-2xx Kubernetes API response.
type StatusError struct {
	Method, Path, Status, Body string
	Code                       int
}

func (e *StatusError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("%s %s: %s", e.Method, e.Path, e.Status)
	}
	return fmt.Sprintf("%s %s: %s: %s", e.Method, e.Path, e.Status, e.Body)
}

func truncate(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) <= maxErrBody {
		return s
	}
	return s[:maxErrBody] + "…"
}
