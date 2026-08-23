package release

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/ianunruh/deploybot/internal/argo"
	"github.com/ianunruh/deploybot/internal/catalog"
	"github.com/ianunruh/deploybot/internal/gitwrite"
	"github.com/ianunruh/deploybot/internal/render"
)

func TestPinDryRunThenApplyAndPromote(t *testing.T) {
	t.Parallel()
	cat := loadExamples(t)
	svc := &Service{Catalog: cat}

	dry, err := svc.Pin(t.Context(), "kmc", "homelab", "ghcr.io/ianunruh/kmc@sha256:abc")
	if err != nil {
		t.Fatal(err)
	}
	if !dry.DryRun || dry.Diff == "" {
		t.Fatalf("expected dry-run diff, got %+v", dry)
	}

	dir := initOpsRepo(t)
	fake := argo.NewFake()
	fake.Set("kmc", argo.Status{Health: "Healthy", Sync: "Synced"})
	apply := &Service{
		Catalog: cat,
		OpsRepo: dir,
		Apply:   true,
		Sync:    true,
		Argo:    argo.StaticRouter{Client: fake},
		Wait:    time.Second,
		Author:  gitwrite.Author{Name: "t", Email: "t@t"},
	}
	pin, err := apply.Pin(t.Context(), "kmc", "homelab", "ghcr.io/ianunruh/kmc:main-dead@sha256:abc")
	if err != nil {
		t.Fatal(err)
	}
	if pin.Commit == "" || !pin.Synced {
		t.Fatalf("pin %+v", pin)
	}

	tree, err := gitwrite.OpenTree(dir)
	if err != nil {
		t.Fatal(err)
	}
	d, err := cat.Get("kmc")
	if err != nil {
		t.Fatal(err)
	}
	img, err := render.CurrentImage(tree, d, "homelab")
	if err != nil {
		t.Fatal(err)
	}
	if img.Digest != "sha256:abc" {
		t.Fatalf("image %+v", img)
	}

	prom, err := apply.Promote(t.Context(), "kmc", "homelab", "prod")
	if err != nil {
		t.Fatal(err)
	}
	if prom.Commit == "" {
		t.Fatal("missing promote commit")
	}
	tree, err = gitwrite.OpenTree(dir)
	if err != nil {
		t.Fatal(err)
	}
	prod, err := render.CurrentImage(tree, d, "prod")
	if err != nil {
		t.Fatal(err)
	}
	if prod != img {
		t.Fatalf("prod %+v want %+v", prod, img)
	}
}

func loadExamples(t *testing.T) *catalog.Catalog {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(file), "..", "..", "examples")
	cat, err := catalog.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	return cat
}

func initOpsRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README"), []byte("ops\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := wt.Add("README"); err != nil {
		t.Fatal(err)
	}
	if _, err := wt.Commit("init", &git.CommitOptions{
		Author: &object.Signature{Name: "t", Email: "t@t", When: time.Now()},
	}); err != nil {
		t.Fatal(err)
	}
	return dir
}
