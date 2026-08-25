package release

import (
	"context"
	"fmt"
	"testing"

	"github.com/ianunruh/deploybot/internal/gitwrite"
	"github.com/ianunruh/deploybot/internal/image"
)

type fakeCompares struct {
	repo, base, head string
	got              image.Compare
	err              error
	hits             int
}

func (f *fakeCompares) CompareCommits(_ context.Context, repoURL, base, head string) (image.Compare, error) {
	f.hits++
	f.repo, f.base, f.head = repoURL, base, head
	return f.got, f.err
}

func changelogService(t *testing.T, cmp *fakeCompares) *Service {
	t.Helper()
	svc := &Service{
		Catalog: loadExamples(t),
		OpsRepo: initOpsRepo(t),
		Apply:   true,
		Author:  gitwrite.Author{Name: "t", Email: "t@t"},
	}
	if cmp != nil {
		svc.Compares = cmp
	}
	return svc
}

func TestChangelogCompare(t *testing.T) {
	t.Parallel()
	cmp := &fakeCompares{got: image.Compare{
		Status:  "ahead",
		AheadBy: 2,
		URL:     "https://github.com/ianunruh/kmc/compare/aaaaaaa...bbbbbbb",
		Commits: []image.Commit{
			{SHA: "bbbbbbb222", Message: "New thing", Author: "Ian Unruh", URL: "https://github.com/ianunruh/kmc/commit/bbbbbbb222"},
			{SHA: "ccccccc333", Message: "Mid thing", Author: "Ian Unruh"},
		},
	}}
	svc := changelogService(t, cmp)
	if _, err := svc.Pin(t.Context(), "kmc", "prod", "ghcr.io/ianunruh/kmc:main-aaaaaaa@sha256:old"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Pin(t.Context(), "kmc", "homelab", "ghcr.io/ianunruh/kmc:main-bbbbbbb@sha256:new"); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.Status(t.Context(), "kmc"); err != nil {
		t.Fatal(err)
	}
	if cmp.hits != 0 {
		t.Fatalf("status should skip compare, hits %d", cmp.hits)
	}

	got, err := svc.Changelog(t.Context(), "kmc", "homelab", "prod")
	if err != nil {
		t.Fatal(err)
	}
	if got.From != "homelab" || got.To != "prod" {
		t.Fatalf("hop %+v", got)
	}
	if got.Status != "ahead" || got.AheadBy != 2 || got.Error != "" {
		t.Fatalf("%+v", got)
	}
	if got.URL != "https://github.com/ianunruh/kmc/compare/aaaaaaa...bbbbbbb" {
		t.Fatalf("url %q", got.URL)
	}
	if got.Head.SHA != "bbbbbbb" || got.Base.SHA != "aaaaaaa" {
		t.Fatalf("pins head=%+v base=%+v", got.Head, got.Base)
	}
	if len(got.Commits) != 2 || got.Commits[0].Message != "New thing" || got.Commits[0].Author != "Ian Unruh" {
		t.Fatalf("commits %+v", got.Commits)
	}
	if cmp.repo != "https://github.com/ianunruh/kmc" || cmp.base != "aaaaaaa" || cmp.head != "bbbbbbb" {
		t.Fatalf("compare args %+v", cmp)
	}
}

func TestChangelogIdentical(t *testing.T) {
	t.Parallel()
	cmp := &fakeCompares{}
	svc := changelogService(t, cmp)
	img := "ghcr.io/ianunruh/kmc:main-bbbbbbb@sha256:abc"
	if _, err := svc.Pin(t.Context(), "kmc", "homelab", img); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Pin(t.Context(), "kmc", "prod", img); err != nil {
		t.Fatal(err)
	}
	got, err := svc.Changelog(t.Context(), "kmc", "homelab", "prod")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "identical" || got.URL == "" || len(got.Commits) != 0 {
		t.Fatalf("%+v", got)
	}
	if cmp.hits != 0 {
		t.Fatalf("identical should skip compare, hits %d", cmp.hits)
	}
}

func TestChangelogFirstPin(t *testing.T) {
	t.Parallel()
	cmp := &fakeCompares{}
	svc := changelogService(t, cmp)
	if _, err := svc.Pin(t.Context(), "kmc", "homelab", "ghcr.io/ianunruh/kmc:main-bbbbbbb@sha256:new"); err != nil {
		t.Fatal(err)
	}
	got, err := svc.Changelog(t.Context(), "kmc", "homelab", "prod")
	if err != nil {
		t.Fatal(err)
	}
	if got.URL != "" || got.Status != "" || len(got.Commits) != 1 {
		t.Fatalf("%+v", got)
	}
	if got.Head.SHA != "bbbbbbb" || got.Commits[0].SHA != "bbbbbbb" {
		t.Fatalf("head %+v", got)
	}
	if cmp.hits != 0 {
		t.Fatalf("unpinned dest should skip compare, hits %d", cmp.hits)
	}
}

func TestChangelogSkipsWithoutSource(t *testing.T) {
	t.Parallel()
	cmp := &fakeCompares{}
	svc := changelogService(t, cmp)
	img := "docker.io/linuxserver/plex:1.43.2.10156-cb3ebc72d@sha256:new"
	if _, err := svc.Pin(t.Context(), "plex", "homelab", img); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Pin(t.Context(), "plex", "prod", "docker.io/linuxserver/plex:1.40.0@sha256:old"); err != nil {
		t.Fatal(err)
	}
	got, err := svc.Changelog(t.Context(), "plex", "homelab", "prod")
	if err != nil {
		t.Fatal(err)
	}
	if got.URL != "" || len(got.Commits) != 0 || got.Head.SHA != "" {
		t.Fatalf("upstream changelog %+v", got)
	}
	if cmp.hits != 0 {
		t.Fatalf("source=false should skip compare, hits %d", cmp.hits)
	}
}

func TestChangelogNoGitSHA(t *testing.T) {
	t.Parallel()
	cmp := &fakeCompares{}
	svc := changelogService(t, cmp)
	img := "docker.io/linuxserver/sonarr:4.0.15.2941-ls285@sha256:abc"
	if _, err := svc.Pin(t.Context(), "sonarr", "homelab", img); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Pin(t.Context(), "sonarr", "prod", "docker.io/linuxserver/sonarr:4.0.14@sha256:old"); err != nil {
		t.Fatal(err)
	}
	got, err := svc.Changelog(t.Context(), "sonarr", "homelab", "prod")
	if err != nil {
		t.Fatal(err)
	}
	if got.URL != "" || len(got.Commits) != 0 || got.Head.SHA != "" {
		t.Fatalf("%+v", got)
	}
	if cmp.hits != 0 {
		t.Fatalf("version tags should skip compare, hits %d", cmp.hits)
	}
}

func TestChangelogUnpinned(t *testing.T) {
	t.Parallel()
	got, err := changelogService(t, &fakeCompares{}).Changelog(t.Context(), "kmc", "homelab", "prod")
	if err != nil {
		t.Fatal(err)
	}
	if got.URL != "" || len(got.Commits) != 0 {
		t.Fatalf("%+v", got)
	}
}

func TestChangelogCompareErrorKeepsURL(t *testing.T) {
	t.Parallel()
	cmp := &fakeCompares{err: fmt.Errorf("github 403: nope")}
	svc := changelogService(t, cmp)
	if _, err := svc.Pin(t.Context(), "kmc", "prod", "ghcr.io/ianunruh/kmc:main-aaaaaaa@sha256:old"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Pin(t.Context(), "kmc", "homelab", "ghcr.io/ianunruh/kmc:main-bbbbbbb@sha256:new"); err != nil {
		t.Fatal(err)
	}
	got, err := svc.Changelog(t.Context(), "kmc", "homelab", "prod")
	if err != nil {
		t.Fatal(err)
	}
	if got.Error == "" || got.URL == "" || len(got.Commits) != 0 {
		t.Fatalf("%+v", got)
	}
}

func TestChangelogCapsCommits(t *testing.T) {
	t.Parallel()
	commits := make([]image.Commit, 0, 25)
	for i := range 25 {
		commits = append(commits, image.Commit{SHA: fmt.Sprintf("%07d", i), Message: fmt.Sprintf("c%d", i)})
	}
	cmp := &fakeCompares{got: image.Compare{Status: "ahead", AheadBy: 25, Commits: commits}}
	svc := changelogService(t, cmp)
	if _, err := svc.Pin(t.Context(), "kmc", "prod", "ghcr.io/ianunruh/kmc:main-aaaaaaa@sha256:old"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Pin(t.Context(), "kmc", "homelab", "ghcr.io/ianunruh/kmc:main-bbbbbbb@sha256:new"); err != nil {
		t.Fatal(err)
	}
	got, err := svc.Changelog(t.Context(), "kmc", "homelab", "prod")
	if err != nil {
		t.Fatal(err)
	}
	if !got.Truncated || got.AheadBy != 25 || len(got.Commits) != maxChangelogCommits {
		t.Fatalf("%+v", got)
	}
}

func TestChangelogUnknown(t *testing.T) {
	t.Parallel()
	svc := changelogService(t, nil)
	if _, err := svc.Changelog(t.Context(), "nope", "homelab", "prod"); err == nil {
		t.Fatal("expected unknown deployable")
	}
	if _, err := svc.Changelog(t.Context(), "kmc", "homelab", "stage-x"); err == nil {
		t.Fatal("expected unknown stage")
	}
}

func TestChangelogWithoutComparesStillHasURL(t *testing.T) {
	t.Parallel()
	svc := changelogService(t, nil)
	if _, err := svc.Pin(t.Context(), "kmc", "prod", "ghcr.io/ianunruh/kmc:main-aaaaaaa@sha256:old"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Pin(t.Context(), "kmc", "homelab", "ghcr.io/ianunruh/kmc:main-bbbbbbb@sha256:new"); err != nil {
		t.Fatal(err)
	}
	got, err := svc.Changelog(t.Context(), "kmc", "homelab", "prod")
	if err != nil {
		t.Fatal(err)
	}
	if got.URL != "https://github.com/ianunruh/kmc/compare/aaaaaaa...bbbbbbb" {
		t.Fatalf("url %q", got.URL)
	}
	if got.Status != "" || len(got.Commits) != 0 {
		t.Fatalf("%+v", got)
	}
}

func TestEqualSHA(t *testing.T) {
	t.Parallel()
	if !equalSHA("b8e5098", "b8e5098") {
		t.Fatal("same short")
	}
	if !equalSHA("b8e509806517abcdef", "b8e5098") {
		t.Fatal("full vs short")
	}
	if equalSHA("b8e509806517abcdef", "b8e509806517ffff") {
		t.Fatal("distinct full")
	}
	if equalSHA("", "b8e5098") {
		t.Fatal("empty")
	}
	if equalSHA("abc", "abcdef0") {
		t.Fatal("short shorter than 7")
	}
}
