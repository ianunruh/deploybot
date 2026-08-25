package release

import (
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"

	"github.com/ianunruh/deploybot/internal/argo"
	"github.com/ianunruh/deploybot/internal/gitwrite"
	"github.com/ianunruh/deploybot/internal/render"
)

func TestRollbackDryRunThenApply(t *testing.T) {
	t.Parallel()
	dir := initOpsRepo(t)
	fake := argo.NewFake()
	fake.Set("kmc", argo.Status{Health: "Healthy", Sync: "Synced"})
	apply := &Service{
		Catalog: loadExamples(t),
		OpsRepo: dir,
		Apply:   true,
		Sync:    true,
		Argo:    argo.StaticRouter{Client: fake},
		Wait:    time.Second,
		Author:  gitwrite.Author{Name: "t", Email: "t@t"},
	}
	good := "ghcr.io/ianunruh/kmc:main-dead@sha256:abc"
	bad := "ghcr.io/ianunruh/kmc:main-bad@sha256:def"
	if _, err := apply.Pin(t.Context(), "kmc", "homelab", good); err != nil {
		t.Fatal(err)
	}
	if _, err := apply.Pin(t.Context(), "kmc", "homelab", bad); err != nil {
		t.Fatal(err)
	}

	dry := &Service{Catalog: apply.Catalog, OpsRepo: dir}
	preview, err := dry.Rollback(t.Context(), "kmc", "homelab", good)
	if err != nil {
		t.Fatal(err)
	}
	if !preview.DryRun || preview.Diff == "" || preview.Commit != "" {
		t.Fatalf("expected dry-run diff, got %+v", preview)
	}

	mut, err := apply.Rollback(t.Context(), "kmc", "homelab", good)
	if err != nil {
		t.Fatal(err)
	}
	if mut.Commit == "" || !mut.Synced {
		t.Fatalf("rollback %+v", mut)
	}
	if len(mut.Files) != 1 || mut.Files[0] != "k8s/kmc/overlays/homelab/kustomization.yaml" {
		t.Fatalf("rollback should only write the homelab overlay, got %v", mut.Files)
	}
	repo, err := git.PlainOpen(dir)
	if err != nil {
		t.Fatal(err)
	}
	c, err := repo.CommitObject(plumbing.NewHash(mut.Commit))
	if err != nil {
		t.Fatal(err)
	}
	if c.Message != "rollback kmc homelab to ghcr.io/ianunruh/kmc:main-dead" {
		t.Fatalf("rollback commit %q", c.Message)
	}

	d := mustKMC(t, apply)
	img, err := render.CurrentImage(mustOpenTree(t, dir), d, "homelab")
	if err != nil {
		t.Fatal(err)
	}
	if img.Digest != "sha256:abc" || img.Tag != "main-dead" {
		t.Fatalf("image %+v", img)
	}

	st, err := apply.Status(t.Context(), "kmc")
	if err != nil {
		t.Fatal(err)
	}
	if st.Stages[0].PreviousRef != bad {
		t.Fatalf("previous %+v", st.Stages[0])
	}
	if st.Stages[0].PreviousImage == "" {
		t.Fatal("expected compact previous image")
	}

	h, err := apply.History(t.Context(), "kmc", 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(h.Events) < 3 || h.Events[0].Kind != EventRollback {
		t.Fatalf("events %+v", h.Events)
	}
}

func TestRollbackRequiresPreviousPin(t *testing.T) {
	t.Parallel()
	dir := initOpsRepo(t)
	svc := &Service{Catalog: loadExamples(t), OpsRepo: dir, Apply: true, Author: gitwrite.Author{Name: "t", Email: "t@t"}}
	_, err := svc.Rollback(t.Context(), "kmc", "homelab", "ghcr.io/ianunruh/kmc:main-dead@sha256:abc")
	if err == nil || !strings.Contains(err.Error(), "no previous pin") {
		t.Fatalf("got %v", err)
	}
	if _, err := svc.Pin(t.Context(), "kmc", "homelab", "ghcr.io/ianunruh/kmc:main-dead@sha256:abc"); err != nil {
		t.Fatal(err)
	}
	_, err = svc.Rollback(t.Context(), "kmc", "homelab", "ghcr.io/ianunruh/kmc:main-dead@sha256:abc")
	if err == nil || !strings.Contains(err.Error(), "already at") {
		t.Fatalf("got %v", err)
	}
	_, err = svc.Rollback(t.Context(), "kmc", "homelab", "ghcr.io/ianunruh/kmc:main-other@sha256:fff")
	if err == nil || !strings.Contains(err.Error(), "no previous pin") {
		t.Fatalf("got %v", err)
	}
}

func TestRollbackUnknownStage(t *testing.T) {
	t.Parallel()
	svc := &Service{Catalog: loadExamples(t)}
	_, err := svc.Rollback(t.Context(), "kmc", "nope", "ghcr.io/ianunruh/kmc@sha256:abc")
	if err == nil || !strings.Contains(err.Error(), "unknown stage") {
		t.Fatalf("got %v", err)
	}
}

func TestRollbackTagOnlyMatchesHistory(t *testing.T) {
	t.Parallel()
	dir := initOpsRepo(t)
	svc := &Service{Catalog: loadExamples(t), OpsRepo: dir, Apply: true, Author: gitwrite.Author{Name: "t", Email: "t@t"}}
	if _, err := svc.Pin(t.Context(), "kmc", "homelab", "ghcr.io/ianunruh/kmc:main-dead@sha256:abc"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Pin(t.Context(), "kmc", "homelab", "ghcr.io/ianunruh/kmc:main-bad@sha256:def"); err != nil {
		t.Fatal(err)
	}
	mut, err := svc.Rollback(t.Context(), "kmc", "homelab", "ghcr.io/ianunruh/kmc:main-dead")
	if err != nil {
		t.Fatal(err)
	}
	if mut.Commit == "" {
		t.Fatal("expected commit")
	}
	img, err := render.CurrentImage(mustOpenTree(t, dir), mustKMC(t, svc), "homelab")
	if err != nil {
		t.Fatal(err)
	}
	if img.Digest != "sha256:abc" {
		t.Fatalf("filled digest %+v", img)
	}
}
