package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
	"time"

	"github.com/ianunruh/deploybot/internal/argo"
	"github.com/ianunruh/deploybot/internal/catalog"
	"github.com/ianunruh/deploybot/internal/config"
	"github.com/ianunruh/deploybot/internal/image"
	"github.com/ianunruh/deploybot/internal/release"
)

func testClusters() map[string]config.Cluster {
	return map[string]config.Cluster{
		"homelab": {
			Headlamp: config.Headlamp{URL: "https://headlamp.k8s.kcloud.zone"},
			Grafana:  config.Grafana{URL: "https://grafana.k8s.kcloud.zone", Logs: true},
		},
		"prod": {
			Headlamp: config.Headlamp{URL: "https://headlamp.k8s.kcloud.io"},
			Grafana:  config.Grafana{URL: "https://grafana.k8s.kcloud.io"},
		},
	}
}

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
		Release: &release.Service{Catalog: cat, Clusters: testClusters()},
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
			Namespace  string `json:"namespace"`
			Project    string `json:"project"`
			Summary    string `json:"summary"`
			DocsURL    string `json:"docsURL"`
			RepoURL    string `json:"repoURL"`
			ProjectURL string `json:"projectURL"`
			Flow       struct {
				Hops []struct {
					From  string `json:"from"`
					To    string `json:"to"`
					State string `json:"state"`
				} `json:"hops"`
			} `json:"flow"`
			Stages []struct {
				Name        string `json:"name"`
				HeadlampURL string `json:"headlampURL"`
				GrafanaURL  string `json:"grafanaURL"`
				LogsURL     string `json:"logsURL"`
			} `json:"stages"`
		} `json:"deployables"`
	}
	if err := json.NewDecoder(res.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(list.Deployables))
	for _, d := range list.Deployables {
		names = append(names, d.Name)
	}
	want := []string{
		"bazarr", "deploybot", "deploybot-valkey", "deploybot-web", "flaresolverr", "humpty",
		"jackett", "kmc", "kmc-controller", "nzbget", "ombi", "plex", "plex-exporter",
		"radarr", "sonarr", "tautulli", "teamspeak", "transmission",
	}
	if !slices.Equal(names, want) {
		t.Fatalf("catalog names %v", names)
	}
	kmc := list.Deployables[7]
	ctrl := list.Deployables[8]
	if kmc.RepoURL != "https://github.com/ianunruh/kmc" {
		t.Fatalf("kmc repo %q", kmc.RepoURL)
	}
	if kmc.ProjectURL != "https://trello.com/b/rPALXxJF/kcloud" {
		t.Fatalf("kmc project %q", kmc.ProjectURL)
	}
	if kmc.Summary == "" {
		t.Fatalf("kmc catalog %+v", kmc)
	}
	if kmc.Namespace != "kmc-system" || kmc.Project != "sandbox" {
		t.Fatalf("kmc namespace/project %+v", kmc)
	}
	sonarr := list.Deployables[14]
	if sonarr.Name != "sonarr" || sonarr.Summary == "" || sonarr.DocsURL == "" {
		t.Fatalf("sonarr catalog %+v", sonarr)
	}
	if sonarr.Namespace != "play" || sonarr.Project != "play" {
		t.Fatalf("sonarr namespace/project %+v", sonarr)
	}
	if ctrl.RepoURL != "https://github.com/ianunruh/kmc" {
		t.Fatalf("controller repo %q", ctrl.RepoURL)
	}
	if list.Deployables[1].RepoURL != "https://github.com/ianunruh/deploybot" {
		t.Fatalf("deploybot repo %q", list.Deployables[1].RepoURL)
	}
	if len(kmc.Stages) != 2 {
		t.Fatalf("kmc stages %+v", kmc.Stages)
	}
	hl := kmc.Stages[0]
	if hl.Name != "homelab" || hl.HeadlampURL == "" || hl.GrafanaURL == "" || hl.LogsURL == "" {
		t.Fatalf("kmc homelab links %+v", hl)
	}
	pr := kmc.Stages[1]
	if pr.Name != "prod" || pr.HeadlampURL == "" || pr.GrafanaURL == "" || pr.LogsURL != "" {
		t.Fatalf("kmc prod links %+v", pr)
	}
	if len(kmc.Flow.Hops) != 1 || kmc.Flow.Hops[0].From != "homelab" || kmc.Flow.Hops[0].To != "prod" {
		t.Fatalf("kmc flow %+v", kmc.Flow)
	}
	if kmc.Flow.Hops[0].State != release.HopCaughtUp {
		t.Fatalf("unpinned kmc hop %q", kmc.Flow.Hops[0].State)
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
	if st.Project != "sandbox" || st.Summary == "" {
		t.Fatalf("status catalog project=%q summary=%q", st.Project, st.Summary)
	}
	if len(st.Stages) != 2 || st.Stages[0].LogsURL == "" || st.Stages[1].LogsURL != "" {
		t.Fatalf("status observability %+v", st.Stages)
	}
}

func TestChangelog(t *testing.T) {
	t.Parallel()
	_, file, _, _ := runtime.Caller(0)
	specs := filepath.Join(filepath.Dir(file), "..", "..", "examples")
	cat, err := catalog.Load(specs)
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{Catalog: cat, Release: &release.Service{Catalog: cat}}
	srv := httptest.NewServer(s.Handler())
	t.Cleanup(srv.Close)

	res, err := http.Get(srv.URL + "/api/v1/deployables/kmc/changelog")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("missing from/to %s", res.Status)
	}

	res2, err := http.Get(srv.URL + "/api/v1/deployables/kmc/changelog?from=homelab&to=prod")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res2.Body.Close() }()
	if res2.StatusCode != http.StatusOK {
		t.Fatal(res2.Status)
	}
	var cl release.Changelog
	if err := json.NewDecoder(res2.Body).Decode(&cl); err != nil {
		t.Fatal(err)
	}
	if cl.From != "homelab" || cl.To != "prod" || cl.Commits == nil {
		t.Fatalf("%+v", cl)
	}

	res3, err := http.Get(srv.URL + "/api/v1/deployables/nope/changelog?from=homelab&to=prod")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res3.Body.Close() }()
	if res3.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown %s", res3.Status)
	}
}

func TestListAndGetDeployedAt(t *testing.T) {
	t.Parallel()
	_, file, _, _ := runtime.Caller(0)
	specs := filepath.Join(filepath.Dir(file), "..", "..", "examples")
	cat, err := catalog.Load(specs)
	if err != nil {
		t.Fatal(err)
	}
	homelabAt := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	prodAt := time.Date(2026, 8, 23, 15, 4, 0, 0, time.UTC)
	homelab := argo.NewFake()
	homelab.Set("kmc", argo.Status{Health: "Healthy", Sync: "Synced", DeployedAt: &homelabAt})
	prod := argo.NewFake()
	prod.Set("kmc", argo.Status{Health: "Healthy", Sync: "Synced", DeployedAt: &prodAt})
	s := &Server{
		Catalog: cat,
		Release: &release.Service{
			Catalog: cat,
			Argo:    listRouter{"homelab": homelab, "prod": prod},
		},
	}
	s.Release.RefreshLive(t.Context())
	srv := httptest.NewServer(s.Handler())
	t.Cleanup(srv.Close)

	res, err := http.Get(srv.URL + "/api/v1/deployables")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	var list struct {
		Deployables []struct {
			Name       string     `json:"name"`
			DeployedAt *time.Time `json:"deployedAt"`
			Flow       struct {
				Hops []struct {
					State string `json:"state"`
				} `json:"hops"`
			} `json:"flow"`
			Stages []struct {
				Name   string `json:"name"`
				Health string `json:"health"`
			} `json:"stages"`
		} `json:"deployables"`
	}
	if err := json.NewDecoder(res.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	idx := -1
	for i, d := range list.Deployables {
		if d.Name == "kmc" {
			idx = i
			break
		}
	}
	if idx < 0 {
		t.Fatal("kmc missing from list")
	}
	kmc := list.Deployables[idx]
	if kmc.DeployedAt == nil || !kmc.DeployedAt.Equal(prodAt) {
		t.Fatalf("list kmc deployedAt %+v", kmc.DeployedAt)
	}
	if len(kmc.Flow.Hops) != 1 || kmc.Flow.Hops[0].State != release.HopCaughtUp {
		t.Fatalf("list kmc flow %+v", kmc.Flow)
	}
	if len(kmc.Stages) != 2 || kmc.Stages[0].Health != "Healthy" || kmc.Stages[1].Health != "Healthy" {
		t.Fatalf("list kmc stages %+v", kmc.Stages)
	}

	res2, err := http.Get(srv.URL + "/api/v1/deployables/kmc")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res2.Body.Close() }()
	var st release.Status
	if err := json.NewDecoder(res2.Body).Decode(&st); err != nil {
		t.Fatal(err)
	}
	if len(st.Stages) != 2 {
		t.Fatalf("stages %+v", st.Stages)
	}
	if st.Stages[0].DeployedAt == nil || !st.Stages[0].DeployedAt.Equal(homelabAt) {
		t.Fatalf("homelab %+v", st.Stages[0].DeployedAt)
	}
	if st.Stages[1].DeployedAt == nil || !st.Stages[1].DeployedAt.Equal(prodAt) {
		t.Fatalf("prod %+v", st.Stages[1].DeployedAt)
	}
}

type listRouter map[string]argo.Client

func (r listRouter) ForStage(stage string) argo.Client { return r[stage] }

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

func TestHistoryEmpty(t *testing.T) {
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

	res, err := http.Get(srv.URL + "/api/v1/deployables/kmc/history")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		t.Fatal(res.Status)
	}
	var h release.History
	if err := json.NewDecoder(res.Body).Decode(&h); err != nil {
		t.Fatal(err)
	}
	if h.Events == nil || h.Releases == nil {
		t.Fatalf("expected empty slices, got %+v", h)
	}
	if len(h.Events) != 0 || len(h.Releases) != 0 {
		t.Fatalf("%+v", h)
	}

	missing, err := http.Get(srv.URL + "/api/v1/deployables/nope/history")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = missing.Body.Close() }()
	if missing.StatusCode != http.StatusNotFound {
		t.Fatalf("status %s", missing.Status)
	}

	global, err := http.Get(srv.URL + "/api/v1/history")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = global.Body.Close() }()
	if global.StatusCode != http.StatusOK {
		t.Fatal(global.Status)
	}
	var all release.GlobalHistory
	if err := json.NewDecoder(global.Body).Decode(&all); err != nil {
		t.Fatal(err)
	}
	if all.Events == nil {
		t.Fatal("expected empty events slice")
	}
	if len(all.Events) != 0 {
		t.Fatalf("%+v", all)
	}

	bad, err := http.Get(srv.URL + "/api/v1/history?limit=nope")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = bad.Body.Close() }()
	if bad.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %s", bad.Status)
	}
}

func TestWorkloadsAndWorkflows(t *testing.T) {
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

	res, err := http.Get(srv.URL + "/api/v1/deployables/kmc/workloads")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		t.Fatal(res.Status)
	}
	var wl release.LiveWorkloads
	if err := json.NewDecoder(res.Body).Decode(&wl); err != nil {
		t.Fatal(err)
	}
	if len(wl.Stages) != 2 || wl.Stages[0].Name != "homelab" || wl.Stages[1].Name != "prod" {
		t.Fatalf("%+v", wl)
	}

	wfRes, err := http.Get(srv.URL + "/api/v1/deployables/kmc/workflows")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = wfRes.Body.Close() }()
	if wfRes.StatusCode != http.StatusOK {
		t.Fatal(wfRes.Status)
	}
	var wf release.Workflows
	if err := json.NewDecoder(wfRes.Body).Decode(&wf); err != nil {
		t.Fatal(err)
	}
	if wf.Runs == nil {
		t.Fatalf("expected empty runs, got %+v", wf)
	}

	missing, err := http.Get(srv.URL + "/api/v1/deployables/nope/workloads")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = missing.Body.Close() }()
	if missing.StatusCode != http.StatusNotFound {
		t.Fatalf("status %s", missing.Status)
	}

	missingWF, err := http.Get(srv.URL + "/api/v1/deployables/nope/workflows")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = missingWF.Body.Close() }()
	if missingWF.StatusCode != http.StatusNotFound {
		t.Fatalf("status %s", missingWF.Status)
	}
}

func TestUpdates(t *testing.T) {
	t.Parallel()
	_, file, _, _ := runtime.Caller(0)
	specs := filepath.Join(filepath.Dir(file), "..", "..", "examples")
	cat, err := catalog.Load(specs)
	if err != nil {
		t.Fatal(err)
	}
	lister := fakeImages{list: release.ImageList{
		Source: "dockerhub",
		Images: []image.Version{{
			Repository: "docker.io/linuxserver/sonarr",
			Ref:        "docker.io/linuxserver/sonarr:4.0.16.2945-ls286@sha256:new",
			Tag:        "4.0.16.2945-ls286",
			Digest:     "sha256:new",
			Tags:       []string{"4.0.16.2945-ls286"},
			CreatedAt:  time.Now().UTC(),
		}},
	}}
	svc := &release.Service{Catalog: cat, Images: lister}
	svc.RefreshUpdates(t.Context())
	s := &Server{Catalog: cat, Release: svc}
	srv := httptest.NewServer(s.Handler())
	t.Cleanup(srv.Close)

	res, err := http.Get(srv.URL + "/api/v1/updates")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		t.Fatal(res.Status)
	}
	var body release.UpdateList
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(body.Updates))
	for _, u := range body.Updates {
		names = append(names, u.Name)
		if u.Name == "kmc" {
			t.Fatal("owned kmc in updates")
		}
	}
	found := false
	for _, u := range body.Updates {
		if u.Name == "sonarr" {
			found = true
			if u.Auto != "24h" || u.Newest == nil || u.Newest.Tag != "4.0.16.2945-ls286" {
				t.Fatalf("sonarr %+v", u)
			}
			if u.Namespace != "play" || u.Project != "play" {
				t.Fatalf("sonarr namespace/project %+v", u)
			}
		}
	}
	if !found {
		t.Fatalf("sonarr missing from %v", names)
	}

	listRes, err := http.Get(srv.URL + "/api/v1/deployables")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listRes.Body.Close() }()
	var list struct {
		Deployables []struct {
			Name   string `json:"name"`
			Update *struct {
				Stale bool   `json:"stale"`
				Auto  string `json:"auto"`
			} `json:"update"`
		} `json:"deployables"`
	}
	if err := json.NewDecoder(listRes.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	var sonarr, kmc bool
	for _, d := range list.Deployables {
		switch d.Name {
		case "sonarr":
			sonarr = true
			if d.Update == nil || d.Update.Auto != "24h" {
				t.Fatalf("sonarr update %+v", d.Update)
			}
		case "kmc":
			kmc = true
			if d.Update != nil {
				t.Fatalf("kmc should not track, got %+v", d.Update)
			}
		}
	}
	if !sonarr || !kmc {
		t.Fatal("missing catalog rows")
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
