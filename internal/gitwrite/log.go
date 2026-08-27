package gitwrite

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"
)

const maxLogWalk = 2000

// Rev is one commit that touched at least one of the requested paths, with
// the blobs of those paths at this commit and at its first parent.
type Rev struct {
	Hash    string
	Message string
	Author  string
	When    time.Time
	Files   map[string][]byte
	Prev    map[string][]byte
}

// HeadHash is the current HEAD commit, or "" if the repo has no commits.
func HeadHash(repoDir string) (string, error) {
	if repoDir == "" {
		return "", nil
	}
	repo, err := git.PlainOpen(repoDir)
	if err != nil {
		return "", fmt.Errorf("open repo %s: %w", repoDir, err)
	}
	head, err := repo.Head()
	if err != nil {
		return "", nil
	}
	return head.Hash().String(), nil
}

// LogPaths returns up to limit commits (newest first) that change any of
// paths. limit <= 0 means no cap. Missing repo or empty history is empty.
//
// Walks first-parent history from HEAD and compares overlay blobs. It does
// not use go-git PathFilter, which diffs every commit in the repo.
func LogPaths(ctx context.Context, repoDir string, paths []string, limit int) ([]Rev, error) {
	revs, _, err := logPaths(ctx, repoDir, paths, limit, "")
	return revs, err
}

// LogPathsAfter returns matching commits newer than sinceHash (exclusive,
// newest first). found is true if sinceHash was reached so the result is a
// complete prefix of history since that commit. A missing sinceHash means
// the caller should rewalk from HEAD.
func LogPathsAfter(ctx context.Context, repoDir string, paths []string, sinceHash string, limit int) ([]Rev, bool, error) {
	if sinceHash == "" {
		revs, err := LogPaths(ctx, repoDir, paths, limit)
		return revs, true, err
	}
	return logPaths(ctx, repoDir, paths, limit, sinceHash)
}

func logPaths(ctx context.Context, repoDir string, paths []string, limit int, sinceHash string) ([]Rev, bool, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if repoDir == "" || len(paths) == 0 {
		return nil, sinceHash == "", nil
	}
	want := make(map[string]struct{}, len(paths))
	for _, p := range paths {
		if p != "" {
			want[p] = struct{}{}
		}
	}
	if len(want) == 0 {
		return nil, sinceHash == "", nil
	}
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	repo, err := git.PlainOpen(repoDir)
	if err != nil {
		return nil, false, fmt.Errorf("open repo %s: %w", repoDir, err)
	}
	head, err := repo.Head()
	if err != nil {
		return nil, sinceHash == "", nil
	}
	iter, err := repo.Log(&git.LogOptions{From: head.Hash()})
	if err != nil {
		return nil, false, fmt.Errorf("log: %w", err)
	}
	defer iter.Close()

	var out []Rev
	walked := 0
	found := sinceHash == ""
	err = iter.ForEach(func(c *object.Commit) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if sinceHash != "" && c.Hash.String() == sinceHash {
			found = true
			return storer.ErrStop
		}
		if limit > 0 && len(out) >= limit {
			return storer.ErrStop
		}
		walked++
		if walked > maxLogWalk {
			return storer.ErrStop
		}
		files, err := blobsAt(c, want)
		if err != nil {
			return err
		}
		prev := map[string][]byte{}
		if c.NumParents() > 0 {
			parent, err := c.Parent(0)
			if err != nil {
				return err
			}
			prev, err = blobsAt(parent, want)
			if err != nil {
				return err
			}
		}
		if !pathsChanged(want, files, prev) {
			return nil
		}
		out = append(out, Rev{
			Hash:    c.Hash.String(),
			Message: c.Message,
			Author:  c.Author.Name,
			When:    c.Author.When.UTC(),
			Files:   files,
			Prev:    prev,
		})
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	if sinceHash != "" && !found {
		return nil, false, nil
	}
	return out, true, nil
}

func pathsChanged(want map[string]struct{}, files, prev map[string][]byte) bool {
	for p := range want {
		if !bytes.Equal(files[p], prev[p]) {
			return true
		}
	}
	return false
}

func blobsAt(c *object.Commit, want map[string]struct{}) (map[string][]byte, error) {
	out := make(map[string][]byte, len(want))
	for p := range want {
		b, err := fileBlob(c, p)
		if err != nil {
			return nil, err
		}
		if b != nil {
			out[p] = b
		}
	}
	return out, nil
}

func fileBlob(c *object.Commit, path string) ([]byte, error) {
	f, err := c.File(path)
	if errors.Is(err, object.ErrFileNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	s, err := f.Contents()
	if err != nil {
		return nil, err
	}
	return []byte(s), nil
}
