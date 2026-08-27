package release

import (
	"strings"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/ianunruh/deploybot/internal/gitwrite"
)

func TestActorGitAuthor(t *testing.T) {
	t.Parallel()
	cases := []struct {
		a          Actor
		name, mail string
	}{
		{Actor{}, "", ""},
		{ActorAutoPin(), "auto-pin", "auto-pin@kcloud.io"},
		{ActorAutoPromote(), "auto-promote", "auto-promote@kcloud.io"},
		{Actor{Kind: ActorKindGitHubActions, ID: "ianunruh", Repo: "ianunruh/kmc"}, "github-actions/ianunruh/kmc", "github-actions@kcloud.io"},
		{Actor{Kind: ActorKindGitHubActions}, "github-actions", "github-actions@kcloud.io"},
		{Actor{Kind: ActorKindUser, ID: "ianunruh", Email: "ian@kcloud.io"}, "ianunruh", "ian@kcloud.io"},
		{Actor{Kind: ActorKindUser, Email: "ian@kcloud.io"}, "ian@kcloud.io", "ian@kcloud.io"},
	}
	for _, tc := range cases {
		got := tc.a.GitAuthor()
		if got.Name != tc.name || got.Email != tc.mail {
			t.Errorf("%+v -> %+v want %s %s", tc.a, got, tc.name, tc.mail)
		}
	}
}

func TestActorAutomated(t *testing.T) {
	t.Parallel()
	if (Actor{}).Automated() || (Actor{Kind: ActorKindUser, ID: "ian"}).Automated() {
		t.Fatal("console/CLI must not be automated")
	}
	if !ActorAutoPin().Automated() || !ActorAutoPromote().Automated() {
		t.Fatal("auto writers must be automated")
	}
	if !(Actor{Kind: ActorKindGitHubActions}).Automated() {
		t.Fatal("github actions must be automated")
	}
}

func TestActorTrailers(t *testing.T) {
	t.Parallel()
	if got := (Actor{}).AppendTrailers("pin kmc homelab to img"); got != "pin kmc homelab to img" {
		t.Fatalf("empty actor %q", got)
	}
	got := Actor{Kind: ActorKindGitHubActions, ID: "ianunruh", Repo: "ianunruh/kmc"}.AppendTrailers("pin kmc homelab to img")
	if !strings.HasPrefix(got, "pin kmc homelab to img\n\n") {
		t.Fatalf("subject %q", got)
	}
	if !strings.Contains(got, "Deploybot-Actor: github-actions\n") {
		t.Fatalf("kind %q", got)
	}
	if !strings.Contains(got, "Deploybot-Actor-ID: ianunruh\n") {
		t.Fatalf("id %q", got)
	}
	if !strings.Contains(got, "Deploybot-Actor-Repo: ianunruh/kmc") {
		t.Fatalf("repo %q", got)
	}
	if eventKind(got) != EventPin {
		t.Fatalf("eventKind %q", eventKind(got))
	}
	parsed := ParseActorTrailers(got)
	if parsed.Kind != ActorKindGitHubActions || parsed.ID != "ianunruh" || parsed.Repo != "ianunruh/kmc" {
		t.Fatalf("parsed %+v", parsed)
	}
	if ParseActorTrailers("pin kmc homelab to img").Kind != "" {
		t.Fatal("subject-only message should have no actor")
	}
}

func TestWithActorPinsAuthorAndTrailer(t *testing.T) {
	t.Parallel()
	dir := initOpsRepo(t)
	svc := &Service{
		Catalog: loadExamples(t),
		OpsRepo: dir,
		Apply:   true,
		Author:  gitwrite.Author{Name: "t", Email: "t@t"},
	}
	if svc.WithActor(ActorAutoPin()) == svc {
		t.Fatal("WithActor should copy")
	}
	if svc.Actor.Kind != "" {
		t.Fatal("process default must stay empty")
	}
	pin, err := svc.WithActor(ActorAutoPin()).Pin(t.Context(), "kmc", "homelab", "ghcr.io/ianunruh/kmc@sha256:abc")
	if err != nil {
		t.Fatal(err)
	}
	c := gitCommit(t, dir, pin.Commit)
	if c.Author.Name != "auto-pin" || c.Author.Email != "auto-pin@kcloud.io" {
		t.Fatalf("author %+v", c.Author)
	}
	if c.Committer.Name != "t" || c.Committer.Email != "t@t" {
		t.Fatalf("committer %+v", c.Committer)
	}
	if !strings.HasPrefix(strings.TrimSpace(c.Message), "pin kmc homelab to ghcr.io/ianunruh/kmc@sha256:abc") {
		t.Fatalf("subject %q", c.Message)
	}
	if !strings.Contains(c.Message, "Deploybot-Actor: auto-pin") {
		t.Fatalf("trailer %q", c.Message)
	}
	if eventKind(c.Message) != EventPin {
		t.Fatalf("eventKind %q", eventKind(c.Message))
	}

	h, err := svc.History(t.Context(), "kmc", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(h.Events) == 0 || h.Events[0].Author != "auto-pin" {
		t.Fatalf("history author %+v", h.Events)
	}
	if h.Events[0].Actor.Kind != ActorKindAutoPin {
		t.Fatalf("history actor %+v", h.Events[0].Actor)
	}
	if len(h.Releases) == 0 {
		t.Fatal("missing release")
	}
	if homelab := h.Releases[0].Stages["homelab"]; homelab.Actor.Kind != ActorKindAutoPin || homelab.Author != "auto-pin" {
		t.Fatalf("release stage %+v", homelab)
	}
}

func TestPinWithoutActorKeepsProcessAuthor(t *testing.T) {
	t.Parallel()
	dir := initOpsRepo(t)
	svc := &Service{
		Catalog: loadExamples(t),
		OpsRepo: dir,
		Apply:   true,
		Author:  gitwrite.Author{Name: "t", Email: "t@t"},
	}
	pin, err := svc.Pin(t.Context(), "kmc", "homelab", "ghcr.io/ianunruh/kmc@sha256:abc")
	if err != nil {
		t.Fatal(err)
	}
	c := gitCommit(t, dir, pin.Commit)
	if c.Author.Name != "t" || c.Committer.Name != "t" {
		t.Fatalf("author %+v committer %+v", c.Author, c.Committer)
	}
	if strings.Contains(c.Message, "Deploybot-Actor:") {
		t.Fatalf("unexpected trailer %q", c.Message)
	}
}

func gitCommit(t *testing.T, dir, hash string) *object.Commit {
	t.Helper()
	repo, err := git.PlainOpen(dir)
	if err != nil {
		t.Fatal(err)
	}
	c, err := repo.CommitObject(plumbing.NewHash(hash))
	if err != nil {
		t.Fatal(err)
	}
	return c
}
