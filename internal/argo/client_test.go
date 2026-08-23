package argo

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ianunruh/deploybot/internal/config"
	"github.com/ianunruh/deploybot/internal/kube"
)

func writeKubeconfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "kubeconfig")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

const testKubeconfig = `
apiVersion: v1
kind: Config
current-context: homelab
clusters:
- name: homelab
  cluster:
    server: https://k8s.example
    insecure-skip-tls-verify: true
contexts:
- name: homelab
  context: {cluster: homelab, user: u}
- name: prod-sjc1
  context: {cluster: homelab, user: u}
users:
- name: u
  user: {token: kube-tok}
`

func TestKubeClientGetAndSync(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /apis/argoproj.io/v1alpha1/namespaces/argocd/applications/kmc", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer kube-tok" {
			t.Errorf("auth %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"metadata": map[string]any{"name": "kmc"},
			"status": map[string]any{
				"health": map[string]any{"status": "Healthy", "message": "ok"},
				"sync":   map[string]any{"status": "Synced", "revision": "abc"},
				"history": []map[string]any{
					{"revision": "old", "deployedAt": "2026-08-01T00:00:00Z"},
					{"revision": "abc", "deployedAt": "2026-08-23T15:04:00Z"},
				},
			},
		})
	})
	mux.HandleFunc("PATCH /apis/argoproj.io/v1alpha1/namespaces/argocd/applications/kmc", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != kube.MergePatch {
			t.Errorf("content-type %q", r.Header.Get("Content-Type"))
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"prune":true`) {
			t.Errorf("body %s", body)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c := &KubeClient{
		REST:      &kube.REST{BaseURL: srv.URL, HTTP: srv.Client(), Auth: kube.Bearer("kube-tok")},
		Namespace: "argocd",
		UIBaseURL: "https://argocd.k8s.kcloud.zone",
	}
	st, err := c.Get(t.Context(), "kmc")
	if err != nil {
		t.Fatal(err)
	}
	if !st.Healthy() || st.Sync != "Synced" || st.Revision != "abc" {
		t.Fatalf("%+v", st)
	}
	want := time.Date(2026, 8, 23, 15, 4, 0, 0, time.UTC)
	if st.DeployedAt == nil || !st.DeployedAt.Equal(want) {
		t.Fatalf("deployedAt %+v", st.DeployedAt)
	}
	if err := c.Sync(t.Context(), "kmc", true); err != nil {
		t.Fatal(err)
	}
	if got := c.AppURL("kmc"); got != "https://argocd.k8s.kcloud.zone/applications/kmc" {
		t.Fatalf("app url %q", got)
	}
}

func TestKubeClientList(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /apis/argoproj.io/v1alpha1/namespaces/argocd/applications", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/apis/argoproj.io/v1alpha1/namespaces/argocd/applications" {
			t.Errorf("path %q", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []map[string]any{
				{
					"metadata": map[string]any{"name": "kmc"},
					"status": map[string]any{
						"health":  map[string]any{"status": "Healthy"},
						"sync":    map[string]any{"status": "Synced", "revision": "abc"},
						"history": []map[string]any{{"deployedAt": "2026-08-23T15:04:00Z"}},
					},
				},
				{
					"metadata": map[string]any{"name": "play-sonarr"},
					"status": map[string]any{
						"health": map[string]any{"status": "Progressing"},
						"sync":   map[string]any{"status": "OutOfSync"},
					},
				},
			},
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c := &KubeClient{
		REST:      &kube.REST{BaseURL: srv.URL, HTTP: srv.Client(), Auth: kube.Bearer("kube-tok")},
		Namespace: "argocd",
	}
	got, err := c.List(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("%+v", got)
	}
	by := map[string]Status{}
	for _, st := range got {
		by[st.Name] = st
	}
	if !by["kmc"].Healthy() || by["kmc"].Revision != "abc" {
		t.Fatalf("kmc %+v", by["kmc"])
	}
	if by["play-sonarr"].Health != "Progressing" || by["play-sonarr"].Sync != "OutOfSync" {
		t.Fatalf("sonarr %+v", by["play-sonarr"])
	}
	want := time.Date(2026, 8, 23, 15, 4, 0, 0, time.UTC)
	if by["kmc"].DeployedAt == nil || !by["kmc"].DeployedAt.Equal(want) {
		t.Fatalf("deployedAt %+v", by["kmc"].DeployedAt)
	}
}

func TestEndpointsFromConfig(t *testing.T) {
	cfg := writeKubeconfig(t, testKubeconfig)
	t.Setenv("KUBECONFIG", cfg)
	t.Setenv("DEPLOYBOT_ARGO_URL", "")
	t.Setenv("DEPLOYBOT_ARGO_URL_HOMELAB", "")
	t.Setenv("DEPLOYBOT_ARGO_URL_PROD", "")

	eps, err := EndpointsFromConfig(map[string]config.Cluster{
		"homelab": {Argo: config.Argo{URL: "https://argocd.k8s.kcloud.zone"}},
		"prod":    {Argo: config.Argo{URL: "https://argocd.k8s.kcloud.io", KubeContext: "prod-sjc1", Namespace: "argocd"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	h, ok := eps.ForStage("homelab").(*KubeClient)
	if !ok || h.Namespace != "argocd" || h.UIBaseURL != "https://argocd.k8s.kcloud.zone" {
		t.Fatalf("homelab %T %+v", eps.ForStage("homelab"), h)
	}
	if got := AppURL(h, "kmc"); got != "https://argocd.k8s.kcloud.zone/applications/kmc" {
		t.Fatalf("homelab app %q", got)
	}
	p, ok := eps.ForStage("prod").(*KubeClient)
	if !ok || p.REST.BaseURL != "https://k8s.example" || p.UIBaseURL != "https://argocd.k8s.kcloud.io" {
		t.Fatalf("prod %T %+v", eps.ForStage("prod"), p)
	}

	t.Setenv("DEPLOYBOT_ARGO_URL_HOMELAB", "https://argo.override.zone")
	eps, err = EndpointsFromConfig(map[string]config.Cluster{
		"homelab": {Argo: config.Argo{URL: "https://argocd.k8s.kcloud.zone"}},
		"prod":    {Argo: config.Argo{URL: "https://argocd.k8s.kcloud.io", KubeContext: "prod-sjc1"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	h = eps.ForStage("homelab").(*KubeClient)
	if h.UIBaseURL != "https://argo.override.zone" {
		t.Fatalf("env url overlay %+v", h)
	}
}

func TestEndpointsSkipsMissingKubeconfig(t *testing.T) {
	t.Setenv("KUBECONFIG", filepath.Join(t.TempDir(), "missing"))
	eps, err := EndpointsFromConfig(map[string]config.Cluster{
		"homelab": {Argo: config.Argo{URL: "https://argocd.k8s.kcloud.zone"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if eps.ForStage("homelab") != nil {
		t.Fatalf("got %+v", eps.ForStage("homelab"))
	}
}

func TestEndpointsSkipsClusterWithoutArgo(t *testing.T) {
	t.Setenv("KUBECONFIG", writeKubeconfig(t, testKubeconfig))
	eps, err := EndpointsFromConfig(map[string]config.Cluster{
		"homelab": {Headlamp: config.Headlamp{URL: "https://headlamp.k8s.kcloud.zone"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if eps.ForStage("homelab") != nil {
		t.Fatalf("got %+v", eps.ForStage("homelab"))
	}
}

func TestEndpointsUnknownKubeContext(t *testing.T) {
	t.Setenv("KUBECONFIG", writeKubeconfig(t, testKubeconfig))
	_, err := EndpointsFromConfig(map[string]config.Cluster{
		"prod": {Argo: config.Argo{URL: "https://argocd.k8s.kcloud.io", KubeContext: "nope"}},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestAppURL(t *testing.T) {
	t.Parallel()
	c := &KubeClient{UIBaseURL: "https://argocd.k8s.kcloud.zone/"}
	if got := c.AppURL("kmc"); got != "https://argocd.k8s.kcloud.zone/applications/kmc" {
		t.Fatalf("got %q", got)
	}
	if got := AppURL(c, "kmc"); got != "https://argocd.k8s.kcloud.zone/applications/kmc" {
		t.Fatalf("helper %q", got)
	}
	if got := (&KubeClient{}).AppURL("kmc"); got != "" {
		t.Fatalf("empty base %q", got)
	}
	if got := AppURL(NewFake(), "kmc"); got != "" {
		t.Fatalf("fake without UI %q", got)
	}
}

func TestDeployedAt(t *testing.T) {
	t.Parallel()
	parse := func(body string) *time.Time {
		t.Helper()
		var raw argoApp
		if err := json.Unmarshal([]byte(body), &raw); err != nil {
			t.Fatal(err)
		}
		return raw.deployedAt()
	}
	eq := func(got *time.Time, want string) {
		t.Helper()
		if want == "" {
			if got != nil {
				t.Fatalf("got %v want nil", got)
			}
			return
		}
		tm, err := time.Parse(time.RFC3339, want)
		if err != nil {
			t.Fatal(err)
		}
		if got == nil || !got.Equal(tm) {
			t.Fatalf("got %v want %s", got, want)
		}
	}

	eq(parse(`{"status":{"history":[
		{"revision":"old","deployedAt":"2026-08-01T00:00:00Z"},
		{"revision":"abc","deployedAt":"2026-08-23T15:04:00Z"}
	]}}`), "2026-08-23T15:04:00Z")
	eq(parse(`{"status":{"history":[
		{"revision":"old","deployedAt":"2026-08-01T00:00:00Z"},
		{"revision":"abc"}
	]}}`), "2026-08-01T00:00:00Z")
	eq(parse(`{"status":{"operationState":{"finishedAt":"2026-08-20T12:00:00Z"}}}`), "2026-08-20T12:00:00Z")
	eq(parse(`{"status":{
		"history":[{"deployedAt":"2026-08-01T00:00:00Z"}],
		"operationState":{"finishedAt":"2026-08-23T15:04:00Z"}
	}}`), "2026-08-01T00:00:00Z")
	eq(parse(`{"status":{}}`), "")
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
