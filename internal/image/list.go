package image

import (
	"cmp"
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
)

const MaxVersions = 50

// Version is a published image that can be pinned.
type Version struct {
	Repository string    `json:"repository"`
	Ref        string    `json:"ref"`
	Tag        string    `json:"tag,omitempty"`
	Digest     string    `json:"digest,omitempty"`
	Tags       []string  `json:"tags"`
	CreatedAt  time.Time `json:"createdAt"`
}

// Listing is a set of pin candidates from one source.
type Listing struct {
	Source   string
	Versions []Version
}

// Lister returns pin candidates for a repository, newest first.
type Lister interface {
	List(ctx context.Context, repository, defaultTag string) (Listing, error)
}

var shaTagRe = regexp.MustCompile(`(?i)^[a-z0-9._-]+-[0-9a-f]{7,40}$`)

// PreferredTag picks an immutable-looking tag over floating names like main/latest.
func PreferredTag(tags []string) string {
	var sha, other, floating string
	for _, t := range tags {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		switch {
		case t == "latest" || t == "main" || t == "master":
			if floating == "" {
				floating = t
			}
		case shaTagRe.MatchString(t):
			if sha == "" {
				sha = t
			}
		default:
			if other == "" {
				other = t
			}
		}
	}
	return cmp.Or(sha, other, floating)
}

func parseGHCR(repository string) (owner, name string, err error) {
	repo := strings.ToLower(strings.TrimSpace(repository))
	repo = strings.TrimPrefix(repo, "https://")
	repo = strings.TrimPrefix(repo, "http://")
	host, rest, ok := strings.Cut(repo, "/")
	if !ok || host != "ghcr.io" {
		return "", "", fmt.Errorf("image listing only supports ghcr.io, got %q", repository)
	}
	owner, name, ok = strings.Cut(rest, "/")
	if !ok || owner == "" || name == "" {
		return "", "", fmt.Errorf("invalid ghcr repository %q", repository)
	}
	return owner, strings.Trim(name, "/"), nil
}
