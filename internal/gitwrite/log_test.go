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

func TestLogPathsImageChanges(t *testing.T) {
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
	write := func(rel, body, msg string) {
		t.Helper()
		fp := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(fp), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fp, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := wt.Add(rel); err != nil {
			t.Fatal(err)
		}
		if _, err := wt.Commit(msg, &git.CommitOptions{
			Author: &object.Signature{Name: "t", Email: "t@t", When: time.Now()},
		}); err != nil {
			t.Fatal(err)
		}
	}
	write("README", "ops\n", "init")
	homelab := "k8s/kmc/overlays/homelab/kustomization.yaml"
	other := "k8s/other/overlays/homelab/kustomization.yaml"
	write(homelab, "resources:\n  - ../../base\n", "add overlay")
	write(other, "resources:\n  - ../../base\n", "unrelated overlay")
	if _, err := Write(dir, render.Tree{
		homelab: []byte("resources:\n  - ../../base\nimages:\n  - name: ghcr.io/ianunruh/kmc\n    digest: sha256:abc\n"),
	}, "pin kmc homelab to ghcr.io/ianunruh/kmc@sha256:abc", Author{Name: "t", Email: "t@t"}); err != nil {
		t.Fatal(err)
	}

	got, err := LogPaths(t.Context(), dir, []string{homelab}, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) < 2 {
		t.Fatalf("want overlay commits, got %d", len(got))
	}
	if got[0].Message != "pin kmc homelab to ghcr.io/ianunruh/kmc@sha256:abc\n" &&
		got[0].Message != "pin kmc homelab to ghcr.io/ianunruh/kmc@sha256:abc" {
		t.Fatalf("newest %q", got[0].Message)
	}
	if string(got[0].Files[homelab]) == string(got[0].Prev[homelab]) {
		t.Fatal("pin should change the overlay blob")
	}
	for _, rev := range got {
		if _, ok := rev.Files[other]; ok {
			t.Fatalf("unrelated path leaked: %+v", rev)
		}
	}
}

func TestLogPathsEmpty(t *testing.T) {
	t.Parallel()
	got, err := LogPaths(t.Context(), "", []string{"a"}, 10)
	if err != nil || got != nil {
		t.Fatalf("%v %+v", err, got)
	}
}
