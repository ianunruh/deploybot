package release

import (
	"context"
	"strings"
	"time"

	"github.com/ianunruh/deploybot/internal/image"
	"github.com/ianunruh/deploybot/internal/render"
)

const (
	changelogTimeout    = 4 * time.Second
	maxChangelogCommits = 20
)

// Changelog is source commits that would ship when promoting from → to.
// Only deployables with spec.links.source are considered; third-party
// images skip this (the overlay still copies the digest). GitHub compare
// is skipped when either pin lacks a SHA.
type Changelog struct {
	From      string         `json:"from"`
	To        string         `json:"to"`
	Base      SourceCommit   `json:"base,omitempty"`
	Head      SourceCommit   `json:"head,omitempty"`
	URL       string         `json:"url,omitempty"`
	Status    string         `json:"status,omitempty"`
	AheadBy   int            `json:"aheadBy,omitempty"`
	BehindBy  int            `json:"behindBy,omitempty"`
	Truncated bool           `json:"truncated,omitempty"`
	Commits   []SourceCommit `json:"commits"`
	Error     string         `json:"error,omitempty"`
}

func (s *Service) Changelog(ctx context.Context, name, from, to string) (Changelog, error) {
	d, err := s.Catalog.Get(name)
	if err != nil {
		return Changelog{}, err
	}
	if _, err := d.Stage(from); err != nil {
		return Changelog{}, err
	}
	if _, err := d.Stage(to); err != nil {
		return Changelog{}, err
	}
	out := Changelog{From: from, To: to, Commits: []SourceCommit{}}
	if !d.HasSourceCommits() {
		return out, nil
	}
	tree, err := s.workingTree(ctx, d)
	if err != nil {
		return Changelog{}, err
	}
	src, err := render.CurrentImage(tree, d, from)
	if err != nil {
		return out, nil
	}
	ctx, cancel := context.WithTimeout(ctx, changelogTimeout)
	defer cancel()
	out.Head = s.resolveSource(ctx, d.Spec.Links.RepoURL, src.Tag)
	headSHA := changelogSHA(out.Head, src.Tag)
	if headSHA == "" {
		return out, nil
	}
	dest, destErr := render.CurrentImage(tree, d, to)
	if destErr != nil {
		if out.Head != (SourceCommit{}) {
			out.Commits = []SourceCommit{out.Head}
		}
		return out, nil
	}
	out.Base = s.resolveSource(ctx, d.Spec.Links.RepoURL, dest.Tag)
	baseSHA := changelogSHA(out.Base, dest.Tag)
	if baseSHA == "" {
		if out.Head != (SourceCommit{}) {
			out.Commits = []SourceCommit{out.Head}
		}
		return out, nil
	}
	out.URL = image.GitHubCompareURL(d.Spec.Links.RepoURL, baseSHA, headSHA)
	if equalSHA(baseSHA, headSHA) {
		out.Status = "identical"
		return out, nil
	}
	if s.Compares == nil {
		return out, nil
	}
	got, err := s.Compares.CompareCommits(ctx, d.Spec.Links.RepoURL, baseSHA, headSHA)
	if err != nil {
		out.Error = err.Error()
		return out, nil
	}
	out.Status = got.Status
	out.AheadBy = got.AheadBy
	out.BehindBy = got.BehindBy
	out.Truncated = got.Truncated
	if got.URL != "" {
		out.URL = got.URL
	}
	for _, c := range got.Commits {
		if len(out.Commits) >= maxChangelogCommits {
			out.Truncated = true
			break
		}
		out.Commits = append(out.Commits, SourceCommit{
			SHA:     c.SHA,
			Message: c.Message,
			Author:  c.Author,
			URL:     c.URL,
		})
	}
	if got.AheadBy > len(out.Commits) {
		out.Truncated = true
	}
	return out, nil
}

func changelogSHA(src SourceCommit, tag string) string {
	if src.SHA != "" {
		return src.SHA
	}
	return image.TagSHA(tag)
}

func equalSHA(a, b string) bool {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	if a == "" || b == "" {
		return false
	}
	if strings.EqualFold(a, b) {
		return true
	}
	if len(a) == len(b) {
		return false
	}
	short, long := a, b
	if len(a) > len(b) {
		short, long = b, a
	}
	return len(short) >= 7 && strings.EqualFold(long[:len(short)], short)
}
