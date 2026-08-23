package gitwrite

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/ianunruh/deploybot/internal/render"
)

func TestWriteCommit(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	readme := filepath.Join(dir, "README")
	if err := os.WriteFile(readme, []byte("ops\n"), 0o644); err != nil {
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

	res, err := Write(dir, render.Tree{
		"k8s/kmc/overlays/homelab/kustomization.yaml": []byte("kind: Kustomization\n"),
	}, "pin kmc homelab", Author{Name: "deploybot", Email: "bot@kcloud.io"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Commit == "" {
		t.Fatal("missing commit")
	}
	b, err := os.ReadFile(filepath.Join(dir, "k8s/kmc/overlays/homelab/kustomization.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "kind: Kustomization\n" {
		t.Fatalf("content %q", b)
	}

	// second write with same content should be a no-op commit reuse
	res2, err := Write(dir, render.Tree{
		"k8s/kmc/overlays/homelab/kustomization.yaml": []byte("kind: Kustomization\n"),
	}, "pin kmc homelab", Author{Name: "deploybot", Email: "bot@kcloud.io"})
	if err != nil {
		t.Fatal(err)
	}
	if res2.Commit != res.Commit {
		t.Fatalf("expected same commit on clean tree, %s vs %s", res.Commit, res2.Commit)
	}
}
