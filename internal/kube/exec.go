package kube

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/ianunruh/deploybot/internal/logx"
)

const execSkew = 10 * time.Second

type execAuth struct {
	mu      sync.Mutex
	cfg     execConfig
	cluster cluster
	token   string
	expiry  time.Time
}

func newExecAuth(cfg execConfig, cluster cluster) *execAuth {
	if cfg.APIVersion == "" {
		cfg.APIVersion = "client.authentication.k8s.io/v1beta1"
	}
	return &execAuth{cfg: cfg, cluster: cluster}
}

func (a *execAuth) Token(ctx context.Context) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.token != "" && (a.expiry.IsZero() || time.Now().Add(execSkew).Before(a.expiry)) {
		return a.token, nil
	}
	tok, expiry, err := a.run(ctx)
	if err != nil {
		return "", err
	}
	a.token = tok
	a.expiry = expiry
	return tok, nil
}

func (a *execAuth) Invalidate() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.token = ""
	a.expiry = time.Time{}
}

type execCredential struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Spec       struct {
		Interactive bool           `json:"interactive"`
		Cluster     map[string]any `json:"cluster,omitempty"`
	} `json:"spec"`
	Status *struct {
		Token                 string `json:"token"`
		ExpirationTimestamp   string `json:"expirationTimestamp"`
		ClientCertificateData string `json:"clientCertificateData"`
	} `json:"status"`
}

func (a *execAuth) run(ctx context.Context) (string, time.Time, error) {
	cred := execCredential{
		APIVersion: a.cfg.APIVersion,
		Kind:       "ExecCredential",
	}
	if a.cfg.ProvideClusterInfo {
		cred.Spec.Cluster = map[string]any{
			"server":                     a.cluster.Server,
			"certificate-authority-data": a.cluster.CertificateAuthorityData,
		}
	}
	info, err := json.Marshal(cred)
	if err != nil {
		return "", time.Time{}, err
	}
	cmd := exec.CommandContext(ctx, a.cfg.Command, a.cfg.Args...)
	cmd.Stdin = bytes.NewReader(info)
	cmd.Env = append(os.Environ(), "KUBERNETES_EXEC_INFO="+string(info))
	for _, e := range a.cfg.Env {
		if e.Name == "" {
			continue
		}
		cmd.Env = append(cmd.Env, e.Name+"="+e.Value)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	start := time.Now()
	err = cmd.Run()
	logx.Done(ctx, "kube exec", start, err, "command", a.cfg.Command)
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", time.Time{}, fmt.Errorf("kube exec %s: %s", a.cfg.Command, msg)
	}
	var out execCredential
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		return "", time.Time{}, fmt.Errorf("kube exec %s: decode: %w", a.cfg.Command, err)
	}
	if out.Status == nil || strings.TrimSpace(out.Status.Token) == "" {
		return "", time.Time{}, fmt.Errorf("kube exec %s: no token in status", a.cfg.Command)
	}
	var expiry time.Time
	if ts := out.Status.ExpirationTimestamp; ts != "" {
		expiry, err = time.Parse(time.RFC3339, ts)
		if err != nil {
			return "", time.Time{}, fmt.Errorf("kube exec %s: expiration: %w", a.cfg.Command, err)
		}
	}
	return strings.TrimSpace(out.Status.Token), expiry, nil
}
