package image

import (
	"context"
	"net/url"
	"strings"
)

// Commit is a source-control commit that produced a release image.
type Commit struct {
	SHA     string `json:"sha,omitempty"`
	Message string `json:"message,omitempty"`
	Author  string `json:"author,omitempty"`
	URL     string `json:"url,omitempty"`
}

// CommitLookup resolves message/author (and a canonical URL) for a SHA in repoURL.
type CommitLookup interface {
	LookupCommit(ctx context.Context, repoURL, sha string) (Commit, error)
}

// TagSHA extracts a git SHA suffix from tags like main-b8e5098.
func TagSHA(tag string) string {
	tag = strings.TrimSpace(tag)
	if !shaTagRe.MatchString(tag) {
		return ""
	}
	i := strings.LastIndex(tag, "-")
	if i < 0 || i+1 >= len(tag) {
		return ""
	}
	return tag[i+1:]
}

// ParseGitHubRepo returns owner and repo for an https GitHub URL.
func ParseGitHubRepo(repoURL string) (owner, repo string, ok bool) {
	u, err := url.Parse(strings.TrimSpace(repoURL))
	if err != nil || u.Host == "" {
		return "", "", false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", "", false
	}
	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	if host != "github.com" {
		return "", "", false
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], strings.TrimSuffix(parts[1], ".git"), true
}

// GitHubCommitURL is the browser commit URL when repoURL is on github.com.
func GitHubCommitURL(repoURL, sha string) string {
	owner, repo, ok := ParseGitHubRepo(repoURL)
	sha = strings.TrimSpace(sha)
	if !ok || sha == "" {
		return ""
	}
	return "https://github.com/" + owner + "/" + repo + "/commit/" + sha
}
