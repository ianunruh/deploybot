package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/ianunruh/deploybot/internal/catalog"
	"github.com/ianunruh/deploybot/internal/image"
	"github.com/ianunruh/deploybot/internal/release"
)

func TestListAndGet(t *testing.T) {
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

	res, err := http.Get(srv.URL + "/api/v1/deployables")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		t.Fatal(res.Status)
	}
	var list struct {
		Deployables []struct {
			Name       string `json:"name"`
			RepoURL    string `json:"repoURL"`
			ProjectURL string `json:"projectURL"`
			StageLinks []struct {
				Name        string `json:"name"`
				HeadlampURL string `json:"headlampURL"`
				GrafanaURL  string `json:"grafanaURL"`
				LogsURL     string `json:"logsURL"`
			} `json:"stageLinks"`
		} `json:"deployables"`
	}
	if err := json.NewDecoder(res.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(list.Deployables))
	for _, d := range list.Deployables {
		names = append(names, d.Name)
	}
	if len(names) != 2 || names[0] != "kmc" || names[1] != "kmc-controller" {
		t.Fatalf("%+v", list)
	}
	if list.Deployables[0].RepoURL != "https://github.com/ianunruh/kmc" {
		t.Fatalf("kmc repo %q", list.Deployables[0].RepoURL)
	}
	if list.Deployables[0].ProjectURL != "https://trello.com/b/rPALXxJF/kcloud" {
		t.Fatalf("kmc project %q", list.Deployables[0].ProjectURL)
	}
	if list.Deployables[1].RepoURL != "https://github.com/ianunruh/kmc" {
		t.Fatalf("controller repo %q", list.Deployables[1].RepoURL)
	}
	if len(list.Deployables[0].StageLinks) != 2 {
		t.Fatalf("kmc stageLinks %+v", list.Deployables[0].StageLinks)
	}
	hl := list.Deployables[0].StageLinks[0]
	if hl.Name != "homelab" || hl.HeadlampURL == "" || hl.GrafanaURL == "" || hl.LogsURL == "" {
		t.Fatalf("kmc homelab links %+v", hl)
	}
	pr := list.Deployables[0].StageLinks[1]
	if pr.Name != "prod" || pr.HeadlampURL == "" || pr.GrafanaURL == "" || pr.LogsURL != "" {
		t.Fatalf("kmc prod links %+v", pr)
	}

	res2, err := http.Get(srv.URL + "/api/v1/deployables/kmc")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res2.Body.Close() }()
	if res2.StatusCode != http.StatusOK {
		t.Fatal(res2.Status)
	}
	var st release.Status
	if err := json.NewDecoder(res2.Body).Decode(&st); err != nil {
		t.Fatal(err)
	}
	if st.RepoURL != "https://github.com/ianunruh/kmc" || st.ProjectURL != "https://trello.com/b/rPALXxJF/kcloud" {
		t.Fatalf("status links %+v", st)
	}
	if len(st.Stages) != 2 || st.Stages[0].LogsURL == "" || st.Stages[1].LogsURL != "" {
		t.Fatalf("status observability %+v", st.Stages)
	}
}

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

func TestListImages(t *testing.T) {
	t.Parallel()
	_, file, _, _ := runtime.Caller(0)
	specs := filepath.Join(filepath.Dir(file), "..", "..", "examples")
	cat, err := catalog.Load(specs)
	if err != nil {
		t.Fatal(err)
	}
	want := release.ImageList{
		Repository: "ghcr.io/ianunruh/kmc",
		Source:     "ghcr",
		Images: []image.Version{
			{
				Repository: "ghcr.io/ianunruh/kmc",
				Ref:        "ghcr.io/ianunruh/kmc:main-b8e5098@sha256:abc",
				Tag:        "main-b8e5098",
				Digest:     "sha256:abc",
				Tags:       []string{"main-b8e5098", "main"},
			},
		},
	}
	s := &Server{
		Catalog: cat,
		Release: &release.Service{Catalog: cat, Images: fakeImages{list: want}},
	}
	srv := httptest.NewServer(s.Handler())
	t.Cleanup(srv.Close)

	res, err := http.Get(srv.URL + "/api/v1/deployables/kmc/images")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		t.Fatal(res.Status)
	}
	var got release.ImageList
	if err := json.NewDecoder(res.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Source != "ghcr" || len(got.Images) != 1 || got.Images[0].Tag != "main-b8e5098" {
		t.Fatalf("%+v", got)
	}

	missing, err := http.Get(srv.URL + "/api/v1/deployables/nope/images")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = missing.Body.Close() }()
	if missing.StatusCode != http.StatusNotFound {
		t.Fatalf("status %s", missing.Status)
	}
}

type fakeImages struct {
	list release.ImageList
	err  error
}

func (f fakeImages) List(_ context.Context, _ string, _ string) (image.Listing, error) {
	if f.err != nil {
		return image.Listing{}, f.err
	}
	return image.Listing{Source: f.list.Source, Versions: f.list.Images}, nil
}
