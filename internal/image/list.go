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

var (
	shaTagRe  = regexp.MustCompile(`(?i)^[a-z0-9._-]+-[0-9a-f]{7,40}$`)
	archTagRe = regexp.MustCompile(`(?i)^(amd64|arm64v8|arm32v7|armhf|arm64|i386)-`)
)

// Registry lists images from GHCR or Docker Hub based on the canonical host.
type Registry struct {
	GitHub    Lister
	DockerHub Lister
}

func (r *Registry) List(ctx context.Context, repository, defaultTag string) (Listing, error) {
	if r == nil {
		return Listing{}, fmt.Errorf("image listing is not configured")
	}
	repo := CanonicalRepository(repository)
	host, _, _ := strings.Cut(repo, "/")
	switch host {
	case "ghcr.io":
		if r.GitHub == nil {
			return Listing{}, fmt.Errorf("ghcr listing is not configured")
		}
		return r.GitHub.List(ctx, repo, defaultTag)
	case "docker.io":
		if r.DockerHub == nil {
			return Listing{}, fmt.Errorf("docker hub listing is not configured")
		}
		return r.DockerHub.List(ctx, repo, defaultTag)
	default:
		return Listing{}, fmt.Errorf("image listing only supports ghcr.io and docker.io, got %q", repository)
	}
}

// CanonicalRepository rewrites lscr.io and implicit Docker Hub names to docker.io/…
func CanonicalRepository(repository string) string {
	repo := strings.ToLower(strings.TrimSpace(repository))
	repo = strings.TrimPrefix(repo, "https://")
	repo = strings.TrimPrefix(repo, "http://")
	repo = strings.TrimSuffix(repo, "/")
	if repo == "" {
		return ""
	}
	host, name := splitRegistry(repo)
	switch host {
	case "lscr.io", "index.docker.io", "registry-1.docker.io", "registry.hub.docker.com", "hub.docker.com":
		host = "docker.io"
	case "":
		host = "docker.io"
	}
	if host == "docker.io" && name != "" && !strings.Contains(name, "/") {
		name = "library/" + name
	}
	if name == "" {
		return host
	}
	return host + "/" + name
}

func splitRegistry(repo string) (host, name string) {
	slash := strings.IndexByte(repo, '/')
	if slash < 0 {
		return "", repo
	}
	first := repo[:slash]
	if strings.ContainsAny(first, ".:") || first == "localhost" {
		return first, strings.Trim(repo[slash+1:], "/")
	}
	return "", repo
}

func skipArchTag(tag string) bool {
	return archTagRe.MatchString(strings.TrimSpace(tag))
}

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
	repo := CanonicalRepository(repository)
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
