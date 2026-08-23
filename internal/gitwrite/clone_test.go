package gitwrite

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func TestCloneAndPull(t *testing.T) {
	t.Parallel()
	_, remote := initLinkedRepos(t)
	dir := t.TempDir()
	dest := filepath.Join(dir, "ops")
	if err := Clone(t.Context(), remote, dest); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest, "README")); err != nil {
		t.Fatal(err)
	}
	if err := Pull(t.Context(), dest); err != nil {
		t.Fatal(err)
	}

	other := t.TempDir()
	if _, err := git.PlainClone(other, false, &git.CloneOptions{URL: remote}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(other, "next"), []byte("n\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	repo, err := git.PlainOpen(other)
	if err != nil {
		t.Fatal(err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := wt.Add("next"); err != nil {
		t.Fatal(err)
	}
	if _, err := wt.Commit("next", &git.CommitOptions{
		Author: &object.Signature{Name: "t", Email: "t@t", When: time.Now()},
	}); err != nil {
		t.Fatal(err)
	}
	if err := repo.Push(&git.PushOptions{RemoteName: git.DefaultRemoteName}); err != nil {
		t.Fatal(err)
	}
	if err := Pull(t.Context(), dest); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest, "next")); err != nil {
		t.Fatal(err)
	}
}

func TestEnsureCloneThenPull(t *testing.T) {
	t.Parallel()
	_, remote := initLinkedRepos(t)
	dest := filepath.Join(t.TempDir(), "ops")
	if err := Ensure(t.Context(), remote, dest); err != nil {
		t.Fatal(err)
	}
	if err := Ensure(t.Context(), remote, dest); err != nil {
		t.Fatal(err)
	}
}

func TestPullNoRemotes(t *testing.T) {
	t.Parallel()
	if err := Pull(t.Context(), initRepo(t)); err != nil {
		t.Fatal(err)
	}
}

func TestEnsureEmptyURL(t *testing.T) {
	t.Parallel()
	if err := Ensure(t.Context(), "", "/nope"); err != nil {
		t.Fatal(err)
	}
}
