package release

import (
	"context"
	"sync"
	"time"

	"github.com/ianunruh/deploybot/internal/image"
)

const sourceCommitTimeout = 2500 * time.Millisecond
const sourceLookupConcurrency = 6

func (s *Service) attachSources(ctx context.Context, repoURL string, releases []Release) {
	if len(releases) == 0 {
		return
	}
	tags := make([]string, 0, len(releases))
	seen := map[string]struct{}{}
	for _, r := range releases {
		if r.Tag == "" {
			continue
		}
		if _, ok := seen[r.Tag]; ok {
			continue
		}
		seen[r.Tag] = struct{}{}
		tags = append(tags, r.Tag)
	}
	if len(tags) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, sourceCommitTimeout)
	defer cancel()
	byTag := make(map[string]SourceCommit, len(tags))
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, sourceLookupConcurrency)
	for _, tag := range tags {
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			src := s.resolveSource(ctx, repoURL, tag)
			if src == (SourceCommit{}) {
				return
			}
			mu.Lock()
			byTag[tag] = src
			mu.Unlock()
		})
	}
	wg.Wait()
	for i := range releases {
		if src, ok := byTag[releases[i].Tag]; ok {
			releases[i].Source = src
		}
	}
}

func (s *Service) resolveSource(ctx context.Context, repoURL, tag string) SourceCommit {
	sha := image.TagSHA(tag)
	if sha == "" {
		return SourceCommit{}
	}
	out := SourceCommit{SHA: sha, URL: image.GitHubCommitURL(repoURL, sha)}
	if s == nil || s.Commits == nil {
		return out
	}
	if err := ctx.Err(); err != nil {
		return out
	}
	got, err := s.Commits.LookupCommit(ctx, repoURL, sha)
	if err != nil {
		return out
	}
	if got.SHA != "" {
		out.SHA = got.SHA
	}
	out.Message = got.Message
	out.Author = got.Author
	if got.URL != "" {
		out.URL = got.URL
	}
	return out
}
