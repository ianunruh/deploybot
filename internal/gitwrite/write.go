package gitwrite

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/ianunruh/deploybot/internal/render"
)

type Author struct {
	Name  string
	Email string
}

func DefaultAuthor() Author {
	return Author{
		Name:  envOr("DEPLOYBOT_GIT_AUTHOR_NAME", "deploybot"),
		Email: envOr("DEPLOYBOT_GIT_AUTHOR_EMAIL", "deploybot@kcloud.io"),
	}
}

func envOr(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}

type Result struct {
	Commit string
	Files  []string
}

// Write materializes tree into repoDir and creates a local commit. It never pushes.
func Write(repoDir string, tree render.Tree, message string, author Author) (Result, error) {
	repo, err := git.PlainOpen(repoDir)
	if err != nil {
		return Result{}, fmt.Errorf("open repo %s: %w", repoDir, err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		return Result{}, fmt.Errorf("worktree: %w", err)
	}

	written := render.SortedPaths(tree)
	for _, rel := range written {
		abs := filepath.Join(repoDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			return Result{}, err
		}
		if err := os.WriteFile(abs, tree[rel], 0o644); err != nil {
			return Result{}, fmt.Errorf("write %s: %w", rel, err)
		}
		if _, err := wt.Add(rel); err != nil {
			return Result{}, fmt.Errorf("git add %s: %w", rel, err)
		}
	}

	st, err := wt.Status()
	if err != nil {
		return Result{}, err
	}
	if st.IsClean() {
		head, err := repo.Head()
		if err != nil {
			return Result{}, err
		}
		return Result{Commit: head.Hash().String(), Files: written}, nil
	}

	hash, err := wt.Commit(message, &git.CommitOptions{
		Author: &object.Signature{
			Name:  author.Name,
			Email: author.Email,
			When:  time.Now(),
		},
	})
	if err != nil {
		return Result{}, fmt.Errorf("commit: %w", err)
	}
	return Result{Commit: hash.String(), Files: written}, nil
}

func OpenTree(repoDir string) (render.Tree, error) {
	tree := render.Tree{}
	err := filepath.WalkDir(repoDir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(repoDir, p)
		if err != nil {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		tree[filepath.ToSlash(rel)] = b
		return nil
	})
	return tree, err
}

// ReadPaths returns the subset of paths that exist under repoDir.
func ReadPaths(repoDir string, paths []string) (render.Tree, error) {
	out := render.Tree{}
	for _, rel := range paths {
		b, err := os.ReadFile(filepath.Join(repoDir, filepath.FromSlash(rel)))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		out[rel] = b
	}
	return out, nil
}
