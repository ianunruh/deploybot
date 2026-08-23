package image

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDockerHubListNewestFirstSkipsArchTags(t *testing.T) {
	t.Parallel()
	var pages int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer hub-token" {
			t.Errorf("auth %s", r.Header.Get("Authorization"))
		}
		if r.URL.Path != "/v2/namespaces/linuxserver/repositories/sonarr/tags" {
			t.Errorf("path %s", r.URL.Path)
		}
		if r.URL.Query().Get("ordering") != "last_updated" {
			t.Errorf("ordering %s", r.URL.Query().Get("ordering"))
		}
		pages++
		writeJSON(t, w, map[string]any{
			"next": nil,
			"results": []map[string]any{
				hubTagJSON("4.0.15.2941-ls285", "2026-08-20T05:16:12Z", "sha256:new"),
				hubTagJSON("amd64-4.0.15.2941-ls285", "2026-08-20T05:16:12Z", "sha256:arch"),
				hubTagJSON("latest", "2026-08-19T00:00:00Z", "sha256:latest"),
				hubTagJSON("arm64v8-4.0.14-ls280", "2026-08-18T00:00:00Z", "sha256:arm"),
				hubTagJSON("4.0.14.2939-ls280", "2026-08-10T00:00:00Z", "abc"),
			},
		})
	}))
	t.Cleanup(srv.Close)

	h := &DockerHub{Token: "hub-token", APIBase: srv.URL, HTTPClient: srv.Client()}
	got, err := h.List(t.Context(), "lscr.io/linuxserver/sonarr", "")
	if err != nil {
		t.Fatal(err)
	}
	if got.Source != "dockerhub" {
		t.Fatalf("source %s", got.Source)
	}
	if pages != 1 {
		t.Fatalf("pages %d", pages)
	}
	if len(got.Versions) != 3 {
		t.Fatalf("versions %+v", got.Versions)
	}
	if got.Versions[0].Tag != "4.0.15.2941-ls285" || got.Versions[0].Digest != "sha256:new" {
		t.Fatalf("newest %+v", got.Versions[0])
	}
	if got.Versions[0].Repository != "docker.io/linuxserver/sonarr" {
		t.Fatalf("repo %s", got.Versions[0].Repository)
	}
	if got.Versions[0].Ref != "docker.io/linuxserver/sonarr:4.0.15.2941-ls285@sha256:new" {
		t.Fatalf("ref %s", got.Versions[0].Ref)
	}
	if got.Versions[1].Tag != "latest" || got.Versions[2].Tag != "4.0.14.2939-ls280" {
		t.Fatalf("order %+v", got.Versions)
	}
	if got.Versions[2].Digest != "sha256:abc" {
		t.Fatalf("digest prefix %+v", got.Versions[2])
	}
}

func TestDockerHubListPagesUntilCap(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.HandleFunc("/v2/namespaces/library/repositories/nginx/tags", func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		if page == "2" {
			writeJSON(t, w, map[string]any{
				"next": nil,
				"results": []map[string]any{
					hubTagJSON("1.27.1", "2026-08-02T00:00:00Z", "sha256:old"),
				},
			})
			return
		}
		next := "http://" + r.Host + "/v2/namespaces/library/repositories/nginx/tags?page=2&page_size=100&ordering=last_updated"
		writeJSON(t, w, map[string]any{
			"next": next,
			"results": []map[string]any{
				hubTagJSON("1.27.2", "2026-08-20T00:00:00Z", "sha256:new"),
			},
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	h := &DockerHub{APIBase: srv.URL, HTTPClient: srv.Client()}
	got, err := h.List(t.Context(), "nginx", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Versions) != 2 {
		t.Fatalf("%+v", got.Versions)
	}
	if got.Versions[0].Tag != "1.27.2" || got.Versions[1].Tag != "1.27.1" {
		t.Fatalf("order %+v", got.Versions)
	}
	if got.Versions[0].Repository != "docker.io/library/nginx" {
		t.Fatalf("repo %s", got.Versions[0].Repository)
	}
}

func TestVersionFromHubFallsBackToAmd64Digest(t *testing.T) {
	t.Parallel()
	got, ok := versionFromHub("docker.io/linuxserver/sonarr", hubTag{
		Name: "4.0.15",
		Images: []hubImage{
			{OS: "unknown", Architecture: "unknown", Digest: "sha256:attest"},
			{OS: "linux", Architecture: "arm64", Digest: "sha256:arm"},
			{OS: "linux", Architecture: "amd64", Digest: "sha256:amd"},
		},
	})
	if !ok || got.Digest != "sha256:amd" {
		t.Fatalf("%+v ok=%v", got, ok)
	}
}

func TestDockerHubListRejectsGHCR(t *testing.T) {
	t.Parallel()
	h := &DockerHub{}
	if _, err := h.List(t.Context(), "ghcr.io/ianunruh/kmc", "main"); err == nil {
		t.Fatal("expected error")
	}
}

func TestDockerHubNotFound(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "object not found"})
	}))
	t.Cleanup(srv.Close)
	h := &DockerHub{APIBase: srv.URL, HTTPClient: srv.Client()}
	_, err := h.List(t.Context(), "docker.io/linuxserver/nope", "")
	if err == nil {
		t.Fatal("expected error")
	}
	if got := err.Error(); got != "docker hub 404: object not found" {
		t.Fatalf("err %s", got)
	}
}

func TestRegistryRoutesLSCRToDockerHub(t *testing.T) {
	t.Parallel()
	hub := &recordLister{}
	gh := &recordLister{}
	r := &Registry{GitHub: gh, DockerHub: hub}
	hub.listing = Listing{Source: "dockerhub", Versions: []Version{{Tag: "v1"}}}
	got, err := r.List(t.Context(), "lscr.io/linuxserver/sonarr", "")
	if err != nil {
		t.Fatal(err)
	}
	if got.Source != "dockerhub" || hub.repo != "docker.io/linuxserver/sonarr" {
		t.Fatalf("hub repo %q listing %+v", hub.repo, got)
	}
	if gh.repo != "" {
		t.Fatalf("github called with %s", gh.repo)
	}
}

func TestRegistryRoutesGHCR(t *testing.T) {
	t.Parallel()
	hub := &recordLister{}
	gh := &recordLister{listing: Listing{Source: "ghcr", Versions: []Version{{Tag: "main"}}}}
	r := &Registry{GitHub: gh, DockerHub: hub}
	got, err := r.List(t.Context(), "ghcr.io/ianunruh/kmc", "main")
	if err != nil {
		t.Fatal(err)
	}
	if got.Source != "ghcr" || gh.repo != "ghcr.io/ianunruh/kmc" {
		t.Fatalf("ghcr repo %q listing %+v", gh.repo, got)
	}
	if hub.repo != "" {
		t.Fatalf("hub called with %s", hub.repo)
	}
}

func TestRegistryRejectsUnknownHost(t *testing.T) {
	t.Parallel()
	r := &Registry{GitHub: &recordLister{}, DockerHub: &recordLister{}}
	if _, err := r.List(t.Context(), "quay.io/acme/app", ""); err == nil {
		t.Fatal("expected error")
	}
}

type recordLister struct {
	repo    string
	listing Listing
}

func (r *recordLister) List(_ context.Context, repository, _ string) (Listing, error) {
	r.repo = repository
	return r.listing, nil
}

func hubTagJSON(name, updated, digest string) map[string]any {
	return map[string]any{
		"name":         name,
		"last_updated": updated,
		"digest":       digest,
	}
}

func TestSkipArchTag(t *testing.T) {
	t.Parallel()
	for _, tag := range []string{"amd64-latest", "arm64v8-1.0", "arm32v7-foo", "armhf-1", "arm64-x", "i386-y"} {
		if !skipArchTag(tag) {
			t.Fatalf("expected skip %s", tag)
		}
	}
	for _, tag := range []string{"latest", "4.0.15.2941-ls285", "v1.2.3", "develop"} {
		if skipArchTag(tag) {
			t.Fatalf("did not expect skip %s", tag)
		}
	}
}

func TestHubCreatedAtParsed(t *testing.T) {
	t.Parallel()
	want := time.Date(2026, 8, 20, 5, 16, 12, 0, time.UTC)
	got := hubTag{}
	if err := json.Unmarshal([]byte(`{"last_updated":"2026-08-20T05:16:12Z"}`), &got); err != nil {
		t.Fatal(err)
	}
	if !want.Equal(got.LastUpdated) {
		t.Fatalf("got %s", got.LastUpdated)
	}
}
