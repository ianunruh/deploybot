package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/ianunruh/deploybot/internal/catalog"
	"github.com/ianunruh/deploybot/internal/release"
)

func TestMutatorSkipSync(t *testing.T) {
	t.Parallel()
	s := &Server{Release: &release.Service{Sync: true}}
	if !s.mutator(nil).Sync {
		t.Fatal("omitted sync should keep the process default")
	}
	on := true
	if !s.mutator(&on).Sync {
		t.Fatal("sync true should keep Argo enabled")
	}
	off := false
	got := s.mutator(&off)
	if got.Sync {
		t.Fatal("sync false should skip Argo")
	}
	if got == s.Release {
		t.Fatal("opt-out should not mutate the process default")
	}
}

func TestReconcileDiffAndPost(t *testing.T) {
	t.Parallel()
	_, file, _, _ := runtime.Caller(0)
	specs := filepath.Join(filepath.Dir(file), "..", "..", "examples")
	cat, err := catalog.Load(specs)
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{
		Catalog: cat,
		Release: &release.Service{Catalog: cat},
	}
	srv := httptest.NewServer(s.Handler())
	t.Cleanup(srv.Close)

	res, err := http.Get(srv.URL + "/api/v1/deployables/kmc/reconcile?stage=homelab")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		t.Fatal(res.Status)
	}
	var preview release.Mutation
	if err := json.NewDecoder(res.Body).Decode(&preview); err != nil {
		t.Fatal(err)
	}
	if !preview.DryRun || preview.Diff == "" || len(preview.Files) == 0 {
		t.Fatalf("%+v", preview)
	}

	body, err := json.Marshal(map[string]string{"stage": "homelab"})
	if err != nil {
		t.Fatal(err)
	}
	res2, err := http.Post(srv.URL+"/api/v1/deployables/kmc/reconcile", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res2.Body.Close() }()
	if res2.StatusCode != http.StatusOK {
		t.Fatal(res2.Status)
	}
	var mut release.Mutation
	if err := json.NewDecoder(res2.Body).Decode(&mut); err != nil {
		t.Fatal(err)
	}
	if !mut.DryRun || mut.Commit != "" {
		t.Fatalf("expected dry-run post, got %+v", mut)
	}

	skip, err := json.Marshal(map[string]any{"stage": "homelab", "sync": false})
	if err != nil {
		t.Fatal(err)
	}
	res3, err := http.Post(srv.URL+"/api/v1/deployables/kmc/reconcile", "application/json", bytes.NewReader(skip))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res3.Body.Close() }()
	if res3.StatusCode != http.StatusOK {
		t.Fatal(res3.Status)
	}

	bad, err := http.Get(srv.URL + "/api/v1/deployables/kmc/reconcile?stage=nope")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = bad.Body.Close() }()
	if bad.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown stage status %s", bad.Status)
	}

	missing, err := http.Get(srv.URL + "/api/v1/deployables/kmc/reconcile")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = missing.Body.Close() }()
	if missing.StatusCode != http.StatusBadRequest {
		t.Fatalf("missing stage status %s", missing.Status)
	}
}
