package image

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestGitHubListPackagesNewestFirst(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("auth %s", r.Header.Get("Authorization"))
		}
		if r.URL.Path != "/users/ianunruh/packages/container/kmc/versions" {
			t.Errorf("path %s", r.URL.Path)
		}
		writeJSON(t, w, []map[string]any{
			pkg("sha256:old", "2026-08-01T00:00:00Z", "main-aaaaaaa"),
			pkg("sha256:untagged", "2026-08-20T05:16:12Z"),
			pkg("sha256:new", "2026-08-20T05:16:12Z", "main", "latest", "main-b8e5098"),
			pkg("sha256:mid", "2026-08-10T00:00:00Z", "main-bbbbbbb"),
		})
	}))
	t.Cleanup(srv.Close)

	g := &GitHub{Token: "test-token", APIBase: srv.URL, HTTPClient: srv.Client()}
	got, err := g.List(t.Context(), "ghcr.io/ianunruh/kmc", "main")
	if err != nil {
		t.Fatal(err)
	}
	if got.Source != "ghcr" {
		t.Fatalf("source %s", got.Source)
	}
	if len(got.Versions) != 3 {
		t.Fatalf("versions %+v", got.Versions)
	}
	if got.Versions[0].Tag != "main-b8e5098" || got.Versions[0].Digest != "sha256:new" {
		t.Fatalf("newest %+v", got.Versions[0])
	}
	if got.Versions[0].Ref != "ghcr.io/ianunruh/kmc:main-b8e5098@sha256:new" {
		t.Fatalf("ref %s", got.Versions[0].Ref)
	}
	if got.Versions[1].Tag != "main-bbbbbbb" || got.Versions[2].Tag != "main-aaaaaaa" {
		t.Fatalf("order %+v", got.Versions)
	}
}

func TestGitHubListUser404ThenOrg(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.HandleFunc("/users/acme/packages/container/app/versions", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
	})
	mux.HandleFunc("/orgs/acme/packages/container/app/versions", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, []map[string]any{
			pkg("sha256:abc", "2026-08-20T00:00:00Z", "v1"),
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	g := &GitHub{Token: "t", APIBase: srv.URL, HTTPClient: srv.Client()}
	got, err := g.List(t.Context(), "ghcr.io/acme/app", "main")
	if err != nil {
		t.Fatal(err)
	}
	if got.Source != "ghcr" || len(got.Versions) != 1 || got.Versions[0].Tag != "v1" {
		t.Fatalf("%+v", got)
	}
}

func TestGitHubPackagesForbiddenFallsBackToCommits(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.HandleFunc("/users/ianunruh/packages/container/kmc/versions", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"message":"You need at least read:packages scope"}`, http.StatusForbidden)
	})
	mux.HandleFunc("/repos/ianunruh/kmc/commits", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, []map[string]any{
			{
				"sha": "b8e509806517abcdef",
				"commit": map[string]any{
					"committer": map[string]any{"date": "2026-08-20T05:15:12Z"},
					"author":    map[string]any{"date": "2026-08-20T05:15:00Z"},
				},
			},
			{
				"sha": "0f733d3deadbeef",
				"commit": map[string]any{
					"committer": map[string]any{"date": "2026-08-20T04:26:00Z"},
				},
			},
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	g := &GitHub{Token: "t", APIBase: srv.URL, HTTPClient: srv.Client()}
	got, err := g.List(t.Context(), "ghcr.io/ianunruh/kmc", "main")
	if err != nil {
		t.Fatal(err)
	}
	if got.Source != "commits" {
		t.Fatalf("source %s", got.Source)
	}
	if len(got.Versions) != 2 {
		t.Fatalf("%+v", got.Versions)
	}
	if got.Versions[0].Tag != "main-b8e5098" || got.Versions[0].Digest != "" {
		t.Fatalf("newest %+v", got.Versions[0])
	}
	if got.Versions[1].Tag != "main-0f733d3" {
		t.Fatalf("second %+v", got.Versions[1])
	}
}

func TestGitHubLookupCommit(t *testing.T) {
	t.Parallel()
	var hits int
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/ianunruh/kmc/commits/b8e5098", func(w http.ResponseWriter, _ *http.Request) {
		hits++
		writeJSON(t, w, map[string]any{
			"sha":      "b8e509806517abcdef",
			"html_url": "https://github.com/ianunruh/kmc/commit/b8e509806517abcdef",
			"commit": map[string]any{
				"message": "Fix the thing\n\nMore detail.",
				"author":  map[string]any{"name": "Ian Unruh", "date": "2026-08-20T05:15:00Z"},
			},
			"author": map[string]any{"login": "ianunruh"},
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	g := &GitHub{Token: "t", APIBase: srv.URL, HTTPClient: srv.Client()}
	got, err := g.LookupCommit(t.Context(), "https://github.com/ianunruh/kmc", "b8e5098")
	if err != nil {
		t.Fatal(err)
	}
	if got.SHA != "b8e509806517abcdef" || got.Message != "Fix the thing" || got.Author != "Ian Unruh" {
		t.Fatalf("%+v", got)
	}
	if got.URL != "https://github.com/ianunruh/kmc/commit/b8e509806517abcdef" {
		t.Fatalf("url %s", got.URL)
	}
	again, err := g.LookupCommit(t.Context(), "https://github.com/ianunruh/kmc", "b8e5098")
	if err != nil {
		t.Fatal(err)
	}
	if again != got {
		t.Fatalf("cache %+v", again)
	}
	if hits != 1 {
		t.Fatalf("hits %d", hits)
	}
	if _, err := g.LookupCommit(t.Context(), "https://gitlab.com/ianunruh/kmc", "b8e5098"); err == nil {
		t.Fatal("expected gitlab error")
	}
}

func TestGitHubListWorkflowRuns(t *testing.T) {
	t.Parallel()
	var hits int
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/ianunruh/kmc/actions/runs", func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.URL.Query().Get("per_page") != "10" {
			t.Errorf("per_page %s", r.URL.Query().Get("per_page"))
		}
		writeJSON(t, w, map[string]any{
			"workflow_runs": []map[string]any{
				{
					"id":            100,
					"name":          "Docker",
					"display_title": "Ship it",
					"run_number":    41,
					"event":         "pull_request",
					"status":        "completed",
					"conclusion":    "success",
					"html_url":      "https://github.com/ianunruh/kmc/actions/runs/100",
					"head_branch":   "main",
					"head_sha":      "aaaaaaaaaaaaaaa",
					"created_at":    "2026-08-23T11:00:00Z",
					"actor":         map[string]any{"login": "ianunruh"},
				},
				{
					"id":             101,
					"name":           "Docker",
					"display_title":  "Fix the thing\n\nMore detail.",
					"run_number":     42,
					"event":          "push",
					"status":         "in_progress",
					"conclusion":     nil,
					"html_url":       "https://github.com/ianunruh/kmc/actions/runs/101",
					"head_branch":    "main",
					"head_sha":       "b8e509806517abcdef",
					"run_started_at": "2026-08-23T12:00:00Z",
					"actor":          map[string]any{"login": "ianunruh"},
					"head_commit":    map[string]any{"message": "Fix the thing"},
				},
				{
					"id":          99,
					"name":        "Test",
					"run_number":  40,
					"event":       "schedule",
					"status":      "completed",
					"conclusion":  "failure",
					"html_url":    "https://github.com/ianunruh/kmc/actions/runs/99",
					"head_branch": "main",
					"head_sha":    "bbbbbbbbbbbbbbb",
					"created_at":  "2026-08-23T10:00:00Z",
				},
			},
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	g := &GitHub{Token: "t", APIBase: srv.URL, HTTPClient: srv.Client()}
	got, err := g.ListWorkflowRuns(t.Context(), "https://github.com/ianunruh/kmc", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("%+v", got)
	}
	inprog := got[0]
	if inprog.ID != 101 || inprog.Name != "Docker" || inprog.Number != 42 {
		t.Fatalf("in progress %+v", inprog)
	}
	if inprog.Status != "in progress" || inprog.Event != "push" || inprog.Actor != "ianunruh" {
		t.Fatalf("in progress fields %+v", inprog)
	}
	if inprog.Title != "Fix the thing" || inprog.Branch != "main" {
		t.Fatalf("in progress title %+v", inprog)
	}
	if inprog.URL != "https://github.com/ianunruh/kmc/actions/runs/101" {
		t.Fatalf("url %s", inprog.URL)
	}
	if inprog.CommitURL != "https://github.com/ianunruh/kmc/commit/b8e509806517abcdef" {
		t.Fatalf("commit %s", inprog.CommitURL)
	}
	if inprog.StartedAt == nil || !inprog.StartedAt.Equal(mustTime("2026-08-23T12:00:00Z")) {
		t.Fatalf("started %+v", inprog.StartedAt)
	}
	if got[1].Status != "success" || got[1].Event != "pull request" {
		t.Fatalf("success %+v", got[1])
	}
	if got[1].StartedAt == nil || !got[1].StartedAt.Equal(mustTime("2026-08-23T11:00:00Z")) {
		t.Fatalf("success started %+v", got[1].StartedAt)
	}
	if got[2].Status != "failure" || got[2].Name != "Test" {
		t.Fatalf("failure %+v", got[2])
	}

	again, err := g.ListWorkflowRuns(t.Context(), "https://github.com/ianunruh/kmc", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 3 {
		t.Fatalf("cache %+v", again)
	}
	if hits != 1 {
		t.Fatalf("hits %d", hits)
	}
	if _, err := g.ListWorkflowRuns(t.Context(), "https://gitlab.com/ianunruh/kmc", 10); err == nil {
		t.Fatal("expected gitlab error")
	}
}

func TestGitHubListWorkflowRunsForbidden(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"message":"Resource not accessible by integration"}`, http.StatusForbidden)
	}))
	t.Cleanup(srv.Close)
	g := &GitHub{Token: "t", APIBase: srv.URL, HTTPClient: srv.Client()}
	_, err := g.ListWorkflowRuns(t.Context(), "https://github.com/ianunruh/kmc", 10)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "actions:read") {
		t.Fatalf("hint %v", err)
	}
}

func TestWorkflowStatus(t *testing.T) {
	t.Parallel()
	cases := []struct {
		status, conclusion, want string
	}{
		{"in_progress", "", "in progress"},
		{"queued", "", "queued"},
		{"completed", "success", "success"},
		{"completed", "failure", "failure"},
		{"completed", "cancelled", "cancelled"},
		{"completed", "timed_out", "timed out"},
		{"completed", "action_required", "action required"},
		{"completed", "", "completed"},
		{"", "", "unknown"},
	}
	for _, tc := range cases {
		if got := workflowStatus(tc.status, tc.conclusion); got != tc.want {
			t.Fatalf("workflowStatus(%q,%q)=%q want %q", tc.status, tc.conclusion, got, tc.want)
		}
	}
}

func TestGitHubLookupCommitNotFoundCached(t *testing.T) {
	t.Parallel()
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)
	g := &GitHub{APIBase: srv.URL, HTTPClient: srv.Client()}
	if _, err := g.LookupCommit(t.Context(), "https://github.com/ianunruh/kmc", "deadbee"); err == nil {
		t.Fatal("expected error")
	}
	if _, err := g.LookupCommit(t.Context(), "https://github.com/ianunruh/kmc", "deadbee"); err == nil {
		t.Fatal("expected cached error")
	}
	if hits != 1 {
		t.Fatalf("hits %d", hits)
	}
}

func TestGitHubListRejectsNonGHCR(t *testing.T) {
	t.Parallel()
	g := &GitHub{Token: "t"}
	if _, err := g.List(t.Context(), "docker.io/library/nginx", "latest"); err == nil {
		t.Fatal("expected error")
	}
}

func TestParseNextLink(t *testing.T) {
	t.Parallel()
	h := `<https://api.github.com/x?page=2>; rel="next", <https://api.github.com/x?page=3>; rel="last"`
	if got := parseNextLink(h); got != "https://api.github.com/x?page=2" {
		t.Fatalf("got %s", got)
	}
	if parseNextLink("") != "" {
		t.Fatal("empty")
	}
}

func TestCreatedAtParsed(t *testing.T) {
	t.Parallel()
	want := time.Date(2026, 8, 20, 5, 16, 12, 0, time.UTC)
	if !want.Equal(mustTime("2026-08-20T05:16:12Z")) {
		t.Fatal("parse")
	}
}

func pkg(name, created string, tags ...string) map[string]any {
	return map[string]any{
		"name":       name,
		"created_at": created,
		"metadata": map[string]any{
			"package_type": "container",
			"container":    map[string]any{"tags": tags},
		},
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, v any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		t.Fatal(err)
	}
}

func mustTime(s string) time.Time {
	tm, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return tm
}

func TestGitHubGetJSONClosesBody(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = io.WriteString(w, `{"message":"nope"}`)
	}))
	t.Cleanup(srv.Close)
	g := &GitHub{APIBase: srv.URL, HTTPClient: srv.Client()}
	var dest []packageVersion
	_, err := g.getJSON(t.Context(), "/x", &dest)
	if err == nil {
		t.Fatal("expected error")
	}
}
