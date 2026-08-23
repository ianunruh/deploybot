package argo

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ianunruh/deploybot/internal/config"
)

func TestHTTPClientGetAndSync(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/applications/kmc", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer tok" {
			t.Errorf("auth %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"metadata": map[string]any{"name": "kmc"},
			"status": map[string]any{
				"health": map[string]any{"status": "Healthy", "message": "ok"},
				"sync":   map[string]any{"status": "Synced", "revision": "abc"},
			},
		})
	})
	mux.HandleFunc("POST /api/v1/applications/kmc/sync", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if string(body) == "" {
			t.Error("empty body")
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c := &HTTPClient{BaseURL: srv.URL, Token: "tok", HTTPClient: srv.Client()}
	st, err := c.Get(t.Context(), "kmc")
	if err != nil {
		t.Fatal(err)
	}
	if !st.Healthy() || st.Sync != "Synced" || st.Revision != "abc" {
		t.Fatalf("%+v", st)
	}
	if err := c.Sync(t.Context(), "kmc", true); err != nil {
		t.Fatal(err)
	}
}

func TestEndpointsFromConfigAndEnv(t *testing.T) {
	dir := t.TempDir()
	tokenPath := filepath.Join(dir, "homelab.token")
	if err := os.WriteFile(tokenPath, []byte(" file-token \n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DEPLOYBOT_ARGO_URL", "")
	t.Setenv("DEPLOYBOT_ARGO_TOKEN", "")
	t.Setenv("DEPLOYBOT_ARGO_URL_HOMELAB", "")
	t.Setenv("DEPLOYBOT_ARGO_TOKEN_HOMELAB", "")
	t.Setenv("DEPLOYBOT_ARGO_URL_PROD", "")
	t.Setenv("DEPLOYBOT_ARGO_TOKEN_PROD", "")
	t.Setenv("YAML_TOKEN", "env-named")

	eps, err := EndpointsFromConfig(map[string]config.Argo{
		"homelab": {URL: "https://argocd.k8s.kcloud.zone", TokenFile: tokenPath},
		"prod":    {URL: "https://argocd.k8s.kcloud.io", TokenEnv: "YAML_TOKEN"},
	})
	if err != nil {
		t.Fatal(err)
	}
	h := eps.ForStage("homelab").(*HTTPClient)
	if h.BaseURL != "https://argocd.k8s.kcloud.zone" || h.Token != "file-token" {
		t.Fatalf("homelab %+v", h)
	}
	if got := h.AppURL("kmc"); got != "https://argocd.k8s.kcloud.zone/applications/kmc" {
		t.Fatalf("homelab app %q", got)
	}
	p := eps.ForStage("prod").(*HTTPClient)
	if p.BaseURL != "https://argocd.k8s.kcloud.io" || p.Token != "env-named" {
		t.Fatalf("prod %+v", p)
	}
	if got := p.AppURL("kmc"); got != "https://argocd.k8s.kcloud.io/applications/kmc" {
		t.Fatalf("prod app %q", got)
	}

	t.Setenv("DEPLOYBOT_ARGO_URL_HOMELAB", "https://argo.override.zone")
	t.Setenv("DEPLOYBOT_ARGO_TOKEN_PROD", "env-wins")
	eps, err = EndpointsFromConfig(map[string]config.Argo{
		"homelab": {URL: "https://argocd.k8s.kcloud.zone", TokenFile: tokenPath},
		"prod":    {URL: "https://argocd.k8s.kcloud.io"},
	})
	if err != nil {
		t.Fatal(err)
	}
	h = eps.ForStage("homelab").(*HTTPClient)
	if h.BaseURL != "https://argo.override.zone" || h.Token != "file-token" {
		t.Fatalf("env url overlay %+v", h)
	}
	p = eps.ForStage("prod").(*HTTPClient)
	if p.Token != "env-wins" {
		t.Fatalf("env token overlay %+v", p)
	}
}

func TestEndpointsFromConfigMissingTokenFile(t *testing.T) {
	t.Parallel()
	_, err := EndpointsFromConfig(map[string]config.Argo{
		"homelab": {URL: "https://argocd.k8s.kcloud.zone", TokenFile: "/nope/token"},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestHTTPClientAppURL(t *testing.T) {
	t.Parallel()
	c := &HTTPClient{BaseURL: "https://argo.kcloud.zone/"}
	if got := c.AppURL("kmc"); got != "https://argo.kcloud.zone/applications/kmc" {
		t.Fatalf("got %q", got)
	}
	if got := AppURL(c, "kmc"); got != "https://argo.kcloud.zone/applications/kmc" {
		t.Fatalf("helper %q", got)
	}
	if got := (&HTTPClient{}).AppURL("kmc"); got != "" {
		t.Fatalf("empty base %q", got)
	}
	if got := AppURL(NewFake(), "kmc"); got != "" {
		t.Fatalf("fake without UI %q", got)
	}
	e := Endpoints{
		"homelab": {BaseURL: "https://argo.kcloud.zone"},
		"":        {BaseURL: "https://argo.kcloud.io"},
	}
	if got := AppURL(e.ForStage("homelab"), "kmc"); got != "https://argo.kcloud.zone/applications/kmc" {
		t.Fatalf("stage %q", got)
	}
	if got := AppURL(e.ForStage("prod"), "kmc"); got != "https://argo.kcloud.io/applications/kmc" {
		t.Fatalf("fallback %q", got)
	}
}

func TestWaitHealthy(t *testing.T) {
	t.Parallel()
	f := NewFake()
	f.Set("kmc", Status{Health: "Progressing", Sync: "Synced"})
	go func() {
		time.Sleep(20 * time.Millisecond)
		f.Set("kmc", Status{Health: "Healthy", Sync: "Synced"})
	}()
	if err := WaitHealthy(t.Context(), f, "kmc", 5*time.Millisecond); err != nil {
		t.Fatal(err)
	}
}

func TestWaitDegraded(t *testing.T) {
	t.Parallel()
	f := NewFake()
	f.Set("kmc", Status{Health: "Degraded", Message: "crash"})
	if err := WaitHealthy(t.Context(), f, "kmc", time.Millisecond); err == nil {
		t.Fatal("expected error")
	}
}
