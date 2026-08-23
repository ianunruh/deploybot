package kube

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"
	"time"
)

func TestResolvePath(t *testing.T) {
	t.Setenv("KUBECONFIG", "")
	if got := ResolvePath("/explicit"); got != "/explicit" {
		t.Fatalf("explicit %q", got)
	}
	t.Setenv("KUBECONFIG", "/a"+string(os.PathListSeparator)+"/b")
	if got := ResolvePath(""); got != "/a" {
		t.Fatalf("env %q", got)
	}
}

func TestLoadRESTTokenGetAnd401Retry(t *testing.T) {
	var hits atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/namespaces/ns/pods", func(w http.ResponseWriter, r *http.Request) {
		n := hits.Add(1)
		if r.Header.Get("Authorization") != "Bearer tok" {
			t.Errorf("auth %q", r.Header.Get("Authorization"))
		}
		if n == 1 {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"reason":"Unauthorized"}`))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"kind": "PodList", "items": []any{}})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c := &REST{BaseURL: srv.URL, HTTP: srv.Client(), Auth: Bearer("tok")}
	var out map[string]any
	if err := c.Get(t.Context(), "/api/v1/namespaces/ns/pods", &out); err != nil {
		t.Fatal(err)
	}
	if out["kind"] != "PodList" {
		t.Fatalf("%v", out)
	}
	if hits.Load() != 2 {
		t.Fatalf("hits %d", hits.Load())
	}
}

func TestRESTPatchAndStatusError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("PATCH /apis/argoproj.io/v1alpha1/namespaces/argocd/applications/kmc", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != MergePatch {
			t.Errorf("content-type %q", r.Header.Get("Content-Type"))
		}
		body, _ := io.ReadAll(r.Body)
		if !json.Valid(body) {
			t.Errorf("body %s", body)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})
	mux.HandleFunc("GET /api/v1/missing", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`not found`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c := &REST{BaseURL: srv.URL, HTTP: srv.Client(), Auth: Bearer("t")}
	if err := c.Patch(t.Context(), "/apis/argoproj.io/v1alpha1/namespaces/argocd/applications/kmc", MergePatch, map[string]any{
		"operation": map[string]any{"sync": map[string]any{"prune": true}},
	}); err != nil {
		t.Fatal(err)
	}
	err := c.Get(t.Context(), "/api/v1/missing", nil)
	var se *StatusError
	if !errors.As(err, &se) || se.Code != 404 {
		t.Fatalf("got %v", err)
	}
}

func TestLoadRESTFromKubeconfig(t *testing.T) {
	dir := t.TempDir()
	tokenPath := filepath.Join(dir, "token")
	if err := os.WriteFile(tokenPath, []byte(" file-tok \n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(dir, "config")
	body := []byte(`
apiVersion: v1
kind: Config
current-context: homelab
clusters:
- name: homelab
  cluster:
    server: https://k8s.example:6443
    insecure-skip-tls-verify: true
contexts:
- name: homelab
  context:
    cluster: homelab
    user: homelab-token
    namespace: kmc-test
- name: other
  context:
    cluster: homelab
    user: missing
users:
- name: homelab-token
  user:
    tokenFile: ` + tokenPath + `
`)
	if err := os.WriteFile(cfg, body, 0o600); err != nil {
		t.Fatal(err)
	}
	rest, err := LoadREST(cfg, "")
	if err != nil {
		t.Fatal(err)
	}
	if rest.BaseURL != "https://k8s.example:6443" {
		t.Fatalf("server %q", rest.BaseURL)
	}
	tok, err := rest.Auth.Token(t.Context())
	if err != nil || tok != "file-tok" {
		t.Fatalf("token %q %v", tok, err)
	}
	_, err = LoadREST(cfg, "nope")
	if !errors.Is(err, ErrNoContext) {
		t.Fatalf("missing context %v", err)
	}
	_, err = LoadREST(filepath.Join(dir, "missing"), "")
	if !errors.Is(err, ErrNoConfig) {
		t.Fatalf("missing file %v", err)
	}
}

func TestExecAuth(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sh helper")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "get-token")
	sh := "#!/bin/sh\n" +
		"if [ -z \"$KUBERNETES_EXEC_INFO\" ]; then echo no-info >&2; exit 1; fi\n" +
		"echo '{\"apiVersion\":\"client.authentication.k8s.io/v1beta1\",\"status\":{\"token\":\"exec-tok\",\"expirationTimestamp\":\"2099-01-01T00:00:00Z\"}}'\n"
	if err := os.WriteFile(script, []byte(sh), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(dir, "config")
	body := []byte(`
apiVersion: v1
kind: Config
current-context: c
clusters:
- name: c
  cluster:
    server: https://k8s.example
    insecure-skip-tls-verify: true
contexts:
- name: c
  context: {cluster: c, user: u}
users:
- name: u
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1beta1
      command: ` + script + `
`)
	if err := os.WriteFile(cfg, body, 0o600); err != nil {
		t.Fatal(err)
	}
	rest, err := LoadREST(cfg, "c")
	if err != nil {
		t.Fatal(err)
	}
	tok, err := rest.Auth.Token(t.Context())
	if err != nil || tok != "exec-tok" {
		t.Fatalf("token %q %v", tok, err)
	}
	tok2, err := rest.Auth.Token(t.Context())
	if err != nil || tok2 != "exec-tok" {
		t.Fatal(err)
	}
}

func TestExecAuthNoToken(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sh helper")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "get-token")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho '{}'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	a := newExecAuth(execConfig{Command: script}, cluster{})
	if _, err := a.Token(t.Context()); err == nil {
		t.Fatal("expected error")
	}
}

func TestDoRejectsRelativePath(t *testing.T) {
	c := &REST{BaseURL: "http://127.0.0.1"}
	if err := c.Get(t.Context(), "pods", nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestBearerEmpty(t *testing.T) {
	tok, err := Bearer("  ").Token(t.Context())
	if err != nil || tok != "" {
		t.Fatalf("%q %v", tok, err)
	}
	Bearer("x").Invalidate()
}

func TestExecExpiryCache(t *testing.T) {
	a := &execAuth{token: "old", expiry: time.Now().Add(-time.Minute)}
	// expired cache must re-run; command missing should error rather than return old.
	a.cfg.Command = "/nope/exec-plugin"
	if _, err := a.Token(t.Context()); err == nil {
		t.Fatal("expected refresh error")
	}
}
