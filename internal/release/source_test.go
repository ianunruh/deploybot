package release

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ianunruh/deploybot/internal/argo"
	"github.com/ianunruh/deploybot/internal/gitwrite"
	"github.com/ianunruh/deploybot/internal/image"
)

type fakeCommits map[string]image.Commit

func (f fakeCommits) LookupCommit(_ context.Context, _ string, sha string) (image.Commit, error) {
	c, ok := f[sha]
	if !ok {
		return image.Commit{}, fmt.Errorf("unknown sha %s", sha)
	}
	return c, nil
}

func TestResolveSourceGitHubURLWithoutLookup(t *testing.T) {
	t.Parallel()
	got := (&Service{}).resolveSource(t.Context(), "https://github.com/ianunruh/kmc", "main-b8e5098")
	if got.SHA != "b8e5098" {
		t.Fatalf("sha %q", got.SHA)
	}
	if got.URL != "https://github.com/ianunruh/kmc/commit/b8e5098" {
		t.Fatalf("url %q", got.URL)
	}
	if got.Message != "" || got.Author != "" {
		t.Fatalf("unexpected %+v", got)
	}
}

func TestResolveSourceSkipsNonGitHubURL(t *testing.T) {
	t.Parallel()
	got := (&Service{}).resolveSource(t.Context(), "https://gitlab.com/ianunruh/kmc", "main-b8e5098")
	if got.SHA != "b8e5098" || got.URL != "" {
		t.Fatalf("%+v", got)
	}
}

func TestResolveSourceLookup(t *testing.T) {
	t.Parallel()
	s := &Service{Commits: fakeCommits{
		"b8e5098": {
			SHA:     "b8e509806517abcdef",
			Message: "Fix the thing",
			Author:  "Ian Unruh",
			URL:     "https://github.com/ianunruh/kmc/commit/b8e509806517abcdef",
		},
	}}
	got := s.resolveSource(t.Context(), "https://github.com/ianunruh/kmc", "main-b8e5098")
	if got.SHA != "b8e509806517abcdef" || got.Message != "Fix the thing" || got.Author != "Ian Unruh" {
		t.Fatalf("%+v", got)
	}
	if got.URL != "https://github.com/ianunruh/kmc/commit/b8e509806517abcdef" {
		t.Fatalf("url %q", got.URL)
	}
}

func TestResolveSourceLookupErrorKeepsURL(t *testing.T) {
	t.Parallel()
	s := &Service{Commits: fakeCommits{}}
	got := s.resolveSource(t.Context(), "https://github.com/ianunruh/kmc", "main-b8e5098")
	if got.SHA != "b8e5098" || got.URL == "" || got.Message != "" {
		t.Fatalf("%+v", got)
	}
}

func TestAttachSources(t *testing.T) {
	t.Parallel()
	releases := []Release{
		{Tag: "main-b8e5098", Image: "main-b8e5098"},
		{Tag: "v1.2.3", Image: "v1.2.3"},
		{Tag: "main-b8e5098", Image: "dup"},
	}
	s := &Service{Commits: fakeCommits{
		"b8e5098": {SHA: "b8e5098", Message: "Fix the thing", Author: "Ian Unruh"},
	}}
	s.attachSources(t.Context(), "https://github.com/ianunruh/kmc", releases)
	if releases[0].Source.Message != "Fix the thing" || releases[0].Source.Author != "Ian Unruh" {
		t.Fatalf("first %+v", releases[0].Source)
	}
	if releases[1].Source != (SourceCommit{}) {
		t.Fatalf("version tag %+v", releases[1].Source)
	}
	if releases[2].Source.Message != "Fix the thing" {
		t.Fatalf("dup %+v", releases[2].Source)
	}
}

func TestHistoryAndStatusSourceCommit(t *testing.T) {
	t.Parallel()
	dir := initOpsRepo(t)
	fake := argo.NewFake()
	fake.Set("kmc", argo.Status{Health: "Healthy", Sync: "Synced"})
	svc := &Service{
		Catalog: loadExamples(t),
		OpsRepo: dir,
		Apply:   true,
		Sync:    true,
		Argo:    argo.StaticRouter{Client: fake},
		Wait:    time.Second,
		Author:  gitwrite.Author{Name: "t", Email: "t@t"},
		Commits: fakeCommits{
			"b8e5098": {
				SHA:     "b8e509806517abcdef",
				Message: "Fix the thing",
				Author:  "Ian Unruh",
				URL:     "https://github.com/ianunruh/kmc/commit/b8e509806517abcdef",
			},
		},
	}
	if _, err := svc.Pin(t.Context(), "kmc", "homelab", "ghcr.io/ianunruh/kmc:main-b8e5098@sha256:abc"); err != nil {
		t.Fatal(err)
	}
	h, err := svc.History(t.Context(), "kmc", 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(h.Releases) != 1 {
		t.Fatalf("releases %+v", h.Releases)
	}
	src := h.Releases[0].Source
	if src.Message != "Fix the thing" || src.Author != "Ian Unruh" || src.SHA != "b8e509806517abcdef" {
		t.Fatalf("history source %+v", src)
	}
	st, err := svc.Status(t.Context(), "kmc")
	if err != nil {
		t.Fatal(err)
	}
	if st.Flow.Source.Message != "Fix the thing" || st.Flow.Source.Author != "Ian Unruh" {
		t.Fatalf("flow source %+v", st.Flow.Source)
	}
}
