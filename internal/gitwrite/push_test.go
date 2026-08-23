package gitwrite

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/ianunruh/deploybot/internal/render"
)

func TestPushUpdatesRemote(t *testing.T) {
	t.Parallel()
	local, remote := initLinkedRepos(t)
	res, err := Write(local, render.Tree{"pin.yaml": []byte("image: v1\n")}, "pin", Author{Name: "t", Email: "t@t"})
	if err != nil {
		t.Fatal(err)
	}
	pushed, err := Push(t.Context(), local)
	if err != nil {
		t.Fatal(err)
	}
	if pushed.Commit != res.Commit {
		t.Fatalf("pushed %s want %s", pushed.Commit, res.Commit)
	}
	if pushed.Remote != git.DefaultRemoteName {
		t.Fatalf("remote %q", pushed.Remote)
	}
	if pushed.Ref() != pushed.Remote+"/"+pushed.Branch {
		t.Fatalf("ref %q", pushed.Ref())
	}
	if got := branchHash(t, remote, pushed.Branch); got != res.Commit {
		t.Fatalf("remote %s want %s", got, res.Commit)
	}

	again, err := Push(t.Context(), local)
	if err != nil {
		t.Fatal(err)
	}
	if again.Commit != res.Commit {
		t.Fatalf("already-up-to-date commit %s", again.Commit)
	}
}

func TestPushRequiresRemote(t *testing.T) {
	t.Parallel()
	dir := initRepo(t)
	_, err := Push(t.Context(), dir)
	if err == nil || !strings.Contains(err.Error(), "no remotes") {
		t.Fatalf("got %v", err)
	}
}

func TestPushRefusesDetachedHead(t *testing.T) {
	t.Parallel()
	local, _ := initLinkedRepos(t)
	repo, err := git.PlainOpen(local)
	if err != nil {
		t.Fatal(err)
	}
	head, err := repo.Head()
	if err != nil {
		t.Fatal(err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	if err := wt.Checkout(&git.CheckoutOptions{Hash: head.Hash()}); err != nil {
		t.Fatal(err)
	}
	_, err = Push(t.Context(), local)
	if err == nil || !strings.Contains(err.Error(), "detached HEAD") {
		t.Fatalf("got %v", err)
	}
}

func TestPushRejectsNonFastForward(t *testing.T) {
	t.Parallel()
	local, remote := initLinkedRepos(t)
	other := t.TempDir()
	if _, err := git.PlainClone(other, false, &git.CloneOptions{URL: remote}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(other, "other"), []byte("theirs\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	otherRepo, err := git.PlainOpen(other)
	if err != nil {
		t.Fatal(err)
	}
	wt, err := otherRepo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := wt.Add("other"); err != nil {
		t.Fatal(err)
	}
	if _, err := wt.Commit("other", &git.CommitOptions{
		Author: &object.Signature{Name: "t", Email: "t@t", When: time.Now()},
	}); err != nil {
		t.Fatal(err)
	}
	if err := otherRepo.Push(&git.PushOptions{RemoteName: git.DefaultRemoteName}); err != nil {
		t.Fatal(err)
	}

	if _, err := Write(local, render.Tree{"ours.yaml": []byte("ours\n")}, "ours", Author{Name: "t", Email: "t@t"}); err != nil {
		t.Fatal(err)
	}
	_, err = Push(t.Context(), local)
	if err == nil || !strings.Contains(err.Error(), "non-fast-forward") {
		t.Fatalf("got %v", err)
	}
}

func initRepo(t *testing.T) string {
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

func initLinkedRepos(t *testing.T) (local, remote string) {
	t.Helper()
	local = initRepo(t)
	remote = addBareRemote(t, local)
	return local, remote
}

func addBareRemote(t *testing.T, local string) string {
	t.Helper()
	remote := filepath.Join(t.TempDir(), "origin.git")
	if _, err := git.PlainInit(remote, true); err != nil {
		t.Fatal(err)
	}
	repo, err := git.PlainOpen(local)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CreateRemote(&config.RemoteConfig{
		Name: git.DefaultRemoteName,
		URLs: []string{remote},
	}); err != nil {
		t.Fatal(err)
	}
	if err := repo.Push(&git.PushOptions{RemoteName: git.DefaultRemoteName}); err != nil {
		t.Fatal(err)
	}
	return remote
}

func branchHash(t *testing.T, repoDir, branch string) string {
	t.Helper()
	repo, err := git.PlainOpen(repoDir)
	if err != nil {
		t.Fatal(err)
	}
	ref, err := repo.Reference(plumbing.NewBranchReferenceName(branch), true)
	if err != nil {
		t.Fatal(err)
	}
	return ref.Hash().String()
}
