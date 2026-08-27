package release

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ianunruh/deploybot/internal/gitwrite"
	"github.com/ianunruh/deploybot/internal/render"
)

func TestPauseHit(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 8, 26, 18, 0, 0, 0, time.UTC)
	file := PauseFile{
		All: &PauseEntry{At: at, By: "ian", Reason: "NYE"},
		Stages: map[string]PauseEntry{
			"prod": {At: at, By: "ian", Reason: "change freeze"},
		},
		Apps: map[string]AppPause{
			"humpty": {At: at, By: "ian", Reason: "crashlooping"},
			"kmc": {
				Stages: map[string]PauseEntry{
					"prod": {At: at, By: "ian", Reason: "hold promote"},
				},
			},
		},
	}
	cases := []struct {
		app, stage, scope, want string
	}{
		{"kmc", "prod", pauseScopeAppStage, "kmc/prod is paused (hold promote)"},
		{"sonarr", "prod", pauseScopeStage, "prod is paused (change freeze)"},
		{"humpty", "homelab", pauseScopeApp, "humpty is paused (crashlooping)"},
		{"humpty", "prod", pauseScopeApp, "humpty is paused (crashlooping)"},
		{"sonarr", "homelab", pauseScopeAll, "deployments paused (NYE)"},
	}

	for _, tc := range cases {
		hit := file.Hit(tc.app, tc.stage)
		if hit == nil {
			t.Fatalf("%s/%s: nil", tc.app, tc.stage)
		}
		if hit.Scope != tc.scope {
			t.Fatalf("%s/%s scope %q want %q", tc.app, tc.stage, hit.Scope, tc.scope)
		}
		if hit.Error() != tc.want {
			t.Fatalf("%s/%s %q", tc.app, tc.stage, hit.Error())
		}
		if !errors.Is(hit, ErrPaused) {
			t.Fatalf("%s/%s unwrap", tc.app, tc.stage)
		}
	}

	onlyProd := PauseFile{Stages: map[string]PauseEntry{"prod": {Reason: "freeze"}}}
	if hit := onlyProd.Hit("kmc", "homelab"); hit != nil {
		t.Fatalf("homelab should be free: %v", hit)
	}
	if hit := onlyProd.Hit("kmc", "prod"); hit == nil || hit.Scope != pauseScopeStage {
		t.Fatalf("prod %+v", hit)
	}
}

func TestPauseSetClear(t *testing.T) {
	t.Parallel()
	e := PauseEntry{At: time.Now().UTC(), By: "ian", Reason: "x"}
	var f PauseFile
	f.set("kmc", "prod", e)
	if !f.has("kmc", "prod") || f.has("kmc", "") || f.has("", "prod") {
		t.Fatalf("app-stage has %+v", f)
	}
	f.set("kmc", "", e)
	if !f.has("kmc", "") || !f.has("kmc", "prod") {
		t.Fatalf("app stacks with app-stage %+v", f)
	}
	f.clear("kmc", "")
	if f.has("kmc", "") || !f.has("kmc", "prod") {
		t.Fatalf("unpause app keeps stage %+v", f)
	}
	f.clear("kmc", "prod")
	if !f.Empty() {
		t.Fatalf("compact %+v", f)
	}
	f.set("", "prod", e)
	f.set("", "", e)
	f.clear("", "")
	if f.All != nil || !f.has("", "prod") {
		t.Fatalf("unpause all keeps stage %+v", f)
	}
}

func TestParsePauseEmpty(t *testing.T) {
	t.Parallel()
	f, err := parsePause(nil)
	if err != nil || !f.Empty() {
		t.Fatalf("%+v %v", f, err)
	}
	f, err = parsePause([]byte(pauseHeader))
	if err != nil || !f.Empty() {
		t.Fatalf("header %+v %v", f, err)
	}
	raw, err := marshalPause(PauseFile{})
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != pauseHeader {
		t.Fatalf("%q", raw)
	}
}

func TestSetPauseBlocksAutomatedPinNotManual(t *testing.T) {
	t.Parallel()
	dir := initOpsRepo(t)
	svc := applySvc(t, dir)
	if _, err := svc.SetPause(t.Context(), "", "", "cluster upgrade"); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, PausePath))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "cluster upgrade") {
		t.Fatalf("%s", raw)
	}
	_, err = svc.WithActor(Actor{Kind: ActorKindGitHubActions, Repo: "ianunruh/kmc"}).Pin(
		t.Context(), "kmc", "homelab", "ghcr.io/ianunruh/kmc@sha256:aaa",
	)
	if !errors.Is(err, ErrPaused) || !strings.Contains(err.Error(), "cluster upgrade") {
		t.Fatalf("ci pin: %v", err)
	}
	_, err = svc.WithActor(ActorAutoPin()).Pin(
		t.Context(), "kmc", "homelab", "ghcr.io/ianunruh/kmc@sha256:aaa",
	)
	if !errors.Is(err, ErrPaused) {
		t.Fatalf("auto-pin: %v", err)
	}
	if _, err := svc.Pin(t.Context(), "kmc", "homelab", "ghcr.io/ianunruh/kmc@sha256:abc"); err != nil {
		t.Fatal(err)
	}
	_, err = svc.WithActor(ActorAutoPromote()).Promote(
		t.Context(), "kmc", "homelab", "prod", "ghcr.io/ianunruh/kmc@sha256:abc",
	)
	if !errors.Is(err, ErrPaused) {
		t.Fatalf("auto-promote: %v", err)
	}
	if _, err := svc.WithActor(Actor{Kind: ActorKindUser, ID: "ian"}).Promote(
		t.Context(), "kmc", "homelab", "prod", "ghcr.io/ianunruh/kmc@sha256:abc",
	); err != nil {
		t.Fatal(err)
	}
}

func TestPauseProdBlocksCIPinToProd(t *testing.T) {
	t.Parallel()
	dir := initOpsRepo(t)
	svc := applySvc(t, dir)
	ci := svc.WithActor(Actor{Kind: ActorKindGitHubActions, Repo: "ianunruh/kmc"})
	if _, err := svc.SetPause(t.Context(), "", "prod", "change freeze"); err != nil {
		t.Fatal(err)
	}
	if _, err := ci.Pin(t.Context(), "kmc", "homelab", "ghcr.io/ianunruh/kmc@sha256:abc"); err != nil {
		t.Fatal(err)
	}
	_, err := ci.Pin(t.Context(), "kmc", "prod", "ghcr.io/ianunruh/kmc@sha256:abc")
	if !errors.Is(err, ErrPaused) || !strings.Contains(err.Error(), "prod is paused") {
		t.Fatalf("ci pin prod: %v", err)
	}
	_, err = svc.WithActor(ActorAutoPromote()).Promote(
		t.Context(), "kmc", "homelab", "prod", "ghcr.io/ianunruh/kmc@sha256:abc",
	)
	if !errors.Is(err, ErrPaused) {
		t.Fatalf("auto-promote: %v", err)
	}
	tree := mustOpenTree(t, dir)
	d := mustKMC(t, svc)
	if _, err := render.CurrentImage(tree, d, "prod"); err == nil {
		t.Fatal("prod should stay unpinned")
	}
}

func TestRollbackWhilePaused(t *testing.T) {
	t.Parallel()
	dir := initOpsRepo(t)
	svc := applySvc(t, dir)
	if _, err := svc.Pin(t.Context(), "kmc", "homelab", "ghcr.io/ianunruh/kmc@sha256:aaa"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Pin(t.Context(), "kmc", "homelab", "ghcr.io/ianunruh/kmc@sha256:bbb"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetPause(t.Context(), "kmc", "", "incident"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.WithActor(Actor{Kind: ActorKindGitHubActions}).Pin(
		t.Context(), "kmc", "homelab", "ghcr.io/ianunruh/kmc@sha256:ccc",
	); err == nil {
		t.Fatal("ci pin should be paused")
	}
	if _, err := svc.Rollback(t.Context(), "kmc", "homelab", "ghcr.io/ianunruh/kmc@sha256:aaa"); err != nil {
		t.Fatal(err)
	}
	tree := mustOpenTree(t, dir)
	d := mustKMC(t, svc)
	img, err := render.CurrentImage(tree, d, "homelab")
	if err != nil {
		t.Fatal(err)
	}
	if img.Digest != "sha256:aaa" {
		t.Fatalf("%+v", img)
	}
}

func TestClearPauseAllowsPin(t *testing.T) {
	t.Parallel()
	dir := initOpsRepo(t)
	svc := applySvc(t, dir)
	if _, err := svc.SetPause(t.Context(), "kmc", "homelab", "hold"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ClearPause(t.Context(), "kmc", "homelab"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Pin(t.Context(), "kmc", "homelab", "ghcr.io/ianunruh/kmc@sha256:abc"); err != nil {
		t.Fatal(err)
	}
	noop, err := svc.ClearPause(t.Context(), "kmc", "homelab")
	if err != nil {
		t.Fatal(err)
	}
	if noop.Commit != "" && noop.Diff != "" {
		t.Fatalf("unpause missing selector should no-op: %+v", noop)
	}
}

func TestPauseUnknownSelector(t *testing.T) {
	t.Parallel()
	svc := &Service{Catalog: loadExamples(t)}
	if _, err := svc.SetPause(t.Context(), "nope", "", ""); err == nil || !strings.Contains(err.Error(), "unknown deployable") {
		t.Fatalf("got %v", err)
	}
	if _, err := svc.SetPause(t.Context(), "kmc", "delta", ""); err == nil || !strings.Contains(err.Error(), "unknown stage") {
		t.Fatalf("got %v", err)
	}
	if _, err := svc.SetPause(t.Context(), "", "delta", ""); err == nil || !strings.Contains(err.Error(), "unknown stage") {
		t.Fatalf("got %v", err)
	}
}

func TestPauseCommitActor(t *testing.T) {
	t.Parallel()
	dir := initOpsRepo(t)
	svc := applySvc(t, dir).WithActor(Actor{Kind: ActorKindUser, ID: "ian", Email: "ian@kcloud.io"})
	mut, err := svc.SetPause(t.Context(), "", "", "NYE")
	if err != nil {
		t.Fatal(err)
	}
	c := gitCommit(t, dir, mut.Commit)
	if c.Author.Name != "ian" {
		t.Fatalf("author %q", c.Author.Name)
	}
	if !strings.Contains(c.Message, "pause all") || !strings.Contains(c.Message, "Deploybot-Actor: user") {
		t.Fatalf("message %q", c.Message)
	}
	if svc.CurrentPause().All == nil || svc.CurrentPause().All.By != "ian" {
		t.Fatalf("%+v", svc.CurrentPause())
	}
}

func applySvc(t *testing.T, dir string) *Service {
	t.Helper()
	return &Service{
		Catalog: loadExamples(t),
		OpsRepo: dir,
		Apply:   true,
		Author:  gitwrite.Author{Name: "t", Email: "t@t"},
	}
}
