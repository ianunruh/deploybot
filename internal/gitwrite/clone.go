package gitwrite

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v5"
)

// Ensure clones url into dir, or pulls if dir is already a git repo.
func Ensure(ctx context.Context, url, dir string) error {
	if url == "" {
		return nil
	}
	if dir == "" {
		return fmt.Errorf("ops repo dir is required to clone %s", url)
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
		return Pull(ctx, dir)
	} else if !os.IsNotExist(err) {
		return err
	}
	return Clone(ctx, url, dir)
}

func Clone(ctx context.Context, url, dir string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	auth, err := authForURL(url)
	if err != nil {
		return err
	}
	_, err = git.PlainCloneContext(ctx, dir, false, &git.CloneOptions{
		URL:  url,
		Auth: auth,
	})
	if err != nil {
		return fmt.Errorf("clone %s: %w", url, err)
	}
	return nil
}

// Pull fast-forwards from the default remote. No remotes and already-up-to-date
// are success so local-only test repos and a freshly cloned tree both work.
func Pull(ctx context.Context, repoDir string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	repo, err := git.PlainOpen(repoDir)
	if err != nil {
		return fmt.Errorf("open repo %s: %w", repoDir, err)
	}
	remotes, err := repo.Remotes()
	if err != nil {
		return fmt.Errorf("remotes: %w", err)
	}
	if len(remotes) == 0 {
		return nil
	}
	wt, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("worktree: %w", err)
	}
	remoteName := git.DefaultRemoteName
	if _, err := repo.Remote(remoteName); err != nil {
		remoteName = remotes[0].Config().Name
	}
	remote, err := repo.Remote(remoteName)
	if err != nil {
		return fmt.Errorf("remote %s: %w", remoteName, err)
	}
	urls := remote.Config().URLs
	if len(urls) == 0 {
		return fmt.Errorf("remote %s has no URL", remoteName)
	}
	auth, err := authForURL(urls[len(urls)-1])
	if err != nil {
		return err
	}
	err = wt.PullContext(ctx, &git.PullOptions{
		RemoteName: remoteName,
		Auth:       auth,
	})
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return fmt.Errorf("pull %s: %w", remoteName, err)
	}
	return nil
}
