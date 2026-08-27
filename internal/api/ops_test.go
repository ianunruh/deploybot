package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ianunruh/deploybot/internal/kube"
	"github.com/ianunruh/deploybot/internal/ops"
)

func TestOpsCatalogWithoutService(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer((&Server{}).Handler())
	t.Cleanup(srv.Close)
	res, err := http.Get(srv.URL + "/api/v1/ops/catalog")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		t.Fatal(res.Status)
	}
	var cat ops.Catalog
	if err := json.NewDecoder(res.Body).Decode(&cat); err != nil {
		t.Fatal(err)
	}
	if len(cat.Kinds) == 0 || cat.Kinds[0].Name != ops.KindPyinfra {
		t.Fatalf("%+v", cat)
	}
}

func TestOpsStartAndLogs(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /apis/batch/v1/namespaces/ops-ci/jobs", func(w http.ResponseWriter, r *http.Request) {
		var job map[string]any
		if err := json.NewDecoder(r.Body).Decode(&job); err != nil {
			t.Fatal(err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"kind": "Job",
			"metadata": map[string]any{
				"name": "ops-api1",
				"labels": map[string]string{
					"app.kubernetes.io/managed-by": "deploybot",
					"deploybot.io/execution":       "true",
					"deploybot.io/kind":            "pyinfra",
					"deploybot.io/cluster":         "homelab",
				},
				"annotations": map[string]string{
					"deploybot.io/dry-run": "true",
					"deploybot.io/summary": "common @ exporter_nodes",
				},
			},
		})
	})
	mux.HandleFunc("GET /apis/batch/v1/namespaces/ops-ci/jobs", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{}})
	})
	mux.HandleFunc("GET /apis/batch/v1/namespaces/ops-ci/jobs/ops-api1", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"kind": "Job",
			"metadata": map[string]any{
				"name": "ops-api1",
				"labels": map[string]string{
					"deploybot.io/execution": "true",
					"deploybot.io/kind":      "pyinfra",
					"deploybot.io/cluster":   "homelab",
				},
			},
			"status": map[string]any{"active": 1},
		})
	})
	mux.HandleFunc("GET /api/v1/namespaces/ops-ci/pods", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []any{
				map[string]any{"metadata": map[string]any{"name": "ops-api1-pod"}},
			},
		})
	})
	mux.HandleFunc("GET /api/v1/namespaces/ops-ci/pods/ops-api1-pod/log", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("log line\n"))
	})
	ks := httptest.NewServer(mux)
	t.Cleanup(ks.Close)

	apiSrv := httptest.NewServer((&Server{
		Ops: &ops.Service{
			Config: ops.Config{
				Namespace: "ops-ci",
				Image:     "ghcr.io/ianunruh/kcloud-ops@sha256:abc",
				RepoURL:   "https://github.com/ianunruh/kcloud-ops",
			},
			Kube:  map[string]*kube.REST{"homelab": {BaseURL: ks.URL, HTTP: ks.Client()}},
			Names: []string{"homelab"},
		},
	}).Handler())
	t.Cleanup(apiSrv.Close)

	res, err := http.Post(apiSrv.URL+"/api/v1/ops/executions", "application/json", strings.NewReader(`{"kind":"pyinfra","cluster":"homelab","params":{"roles":["common"],"limit":"exporter_nodes"}}`))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("start %s %s", res.Status, body)
	}
	var ex ops.Execution
	if err := json.NewDecoder(res.Body).Decode(&ex); err != nil {
		t.Fatal(err)
	}
	if ex.ID != "ops-api1" {
		t.Fatalf("%+v", ex)
	}

	logRes, err := http.Get(apiSrv.URL + "/api/v1/ops/executions/ops-api1/logs?cluster=homelab")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = logRes.Body.Close() }()
	body, _ := io.ReadAll(logRes.Body)
	if logRes.StatusCode != http.StatusOK || string(body) != "log line\n" {
		t.Fatalf("logs %s %q", logRes.Status, body)
	}
}

func TestOpsStartNoImage(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer((&Server{
		Ops: &ops.Service{Names: []string{"homelab"}, Kube: map[string]*kube.REST{"homelab": {}}},
	}).Handler())
	t.Cleanup(srv.Close)
	res, err := http.Post(srv.URL+"/api/v1/ops/executions", "application/json", strings.NewReader(`{"kind":"pyinfra","cluster":"homelab","params":{"roles":["common"]}}`))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusServiceUnavailable {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("status %s %s", res.Status, body)
	}
}
