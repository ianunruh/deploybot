package image

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ianunruh/deploybot/internal/logx"
)

const (
	githubAPIVersion    = "2022-11-28"
	maxPackagePages     = 2
	defaultWorkflowRuns = 10
	maxWorkflowRuns     = 30
	workflowRunsTTL     = 15 * time.Second
)

// GitHub lists GHCR images through the GitHub Packages API and looks up
// source commits on github.com.
type GitHub struct {
	Token      string
	APIBase    string
	HTTPClient *http.Client

	mu      sync.Mutex
	commits map[string]cachedCommit
	runs    map[string]cachedRuns
}

type cachedCommit struct {
	commit Commit
	found  bool
}

type cachedRuns struct {
	runs []WorkflowRun
	err  error
	at   time.Time
}

func NewGitHub() *GitHub {
	token, _ := ResolveToken()
	return &GitHub{
		Token:      token,
		APIBase:    "https://api.github.com",
		HTTPClient: &http.Client{Timeout: 20 * time.Second},
	}
}

// ResolveToken returns a GitHub token and where it came from.
func ResolveToken() (token, source string) {
	for _, e := range []string{"DEPLOYBOT_GITHUB_TOKEN", "GH_TOKEN", "GITHUB_TOKEN"} {
		if v := strings.TrimSpace(os.Getenv(e)); v != "" {
			return v, e
		}
	}
	if t := ghAuthToken(); t != "" {
		return t, "gh"
	}
	return "", "none"
}

func ghAuthToken() string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "gh", "auth", "token")
	start := time.Now()
	out, err := cmd.Output()
	logx.Done(ctx, "gh auth token", start, err)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func (g *GitHub) List(ctx context.Context, repository, defaultTag string) (Listing, error) {
	owner, pkg, err := parseGHCR(repository)
	if err != nil {
		return Listing{}, err
	}
	vers, err := g.packageVersions(ctx, owner, pkg, repository)
	if err == nil {
		return Listing{Source: "ghcr", Versions: vers}, nil
	}
	commits, err2 := g.commitVersions(ctx, owner, pkg, repository, defaultTag)
	if err2 == nil && len(commits) > 0 {
		return Listing{Source: "commits", Versions: commits}, nil
	}
	if err2 != nil {
		return Listing{}, errors.Join(
			fmt.Errorf("ghcr packages: %w", err),
			fmt.Errorf("git commits: %w", err2),
		)
	}
	return Listing{}, fmt.Errorf("ghcr packages: %w", err)
}

func (g *GitHub) packageVersions(ctx context.Context, owner, pkg, repository string) ([]Version, error) {
	raw, err := g.listPackagePages(ctx, packagePath("users", owner, pkg))
	var apiErr *apiError
	if errors.As(err, &apiErr) && apiErr.Status == http.StatusNotFound {
		raw, err = g.listPackagePages(ctx, packagePath("orgs", owner, pkg))
	}
	if err != nil {
		return nil, authHint(err, "token needs read:packages to list GHCR versions")
	}
	out := make([]Version, 0, len(raw))
	for _, v := range raw {
		ver, ok := versionFromPackage(repository, v)
		if !ok {
			continue
		}
		out = append(out, ver)
	}
	sortVersions(out)
	if len(out) > MaxVersions {
		out = out[:MaxVersions]
	}
	return out, nil
}

func packagePath(kind, owner, pkg string) string {
	return fmt.Sprintf("/%s/%s/packages/container/%s/versions?per_page=100",
		kind, url.PathEscape(owner), url.PathEscape(pkg))
}

func (g *GitHub) listPackagePages(ctx context.Context, path string) ([]packageVersion, error) {
	var all []packageVersion
	next := path
	for range maxPackagePages {
		if next == "" {
			break
		}
		var batch []packageVersion
		link, err := g.getJSON(ctx, next, &batch)
		if err != nil {
			return nil, err
		}
		all = append(all, batch...)
		next = parseNextLink(link)
	}
	return all, nil
}

func (g *GitHub) commitVersions(ctx context.Context, owner, pkg, repository, defaultTag string) ([]Version, error) {
	if strings.Contains(pkg, "/") {
		return nil, fmt.Errorf("no git repo inferred for %s/%s", owner, pkg)
	}
	prefix := cmp.Or(defaultTag, "main")
	path := fmt.Sprintf("/repos/%s/%s/commits?per_page=%d", url.PathEscape(owner), url.PathEscape(pkg), MaxVersions)
	var commits []gitCommit
	if _, err := g.getJSON(ctx, path, &commits); err != nil {
		return nil, err
	}
	out := make([]Version, 0, len(commits))
	for _, c := range commits {
		if len(c.SHA) < 7 {
			continue
		}
		tag := prefix + "-" + c.SHA[:7]
		created := c.Commit.Committer.Date
		if created.IsZero() {
			created = c.Commit.Author.Date
		}
		ref := Ref{Repository: repository, Tag: tag}
		out = append(out, Version{
			Repository: repository,
			Ref:        ref.String(),
			Tag:        tag,
			Tags:       []string{tag},
			CreatedAt:  created,
		})
	}
	sortVersions(out)
	return out, nil
}

// LookupCommit fetches a commit from a github.com repoURL. Results are cached
// for the process lifetime; 404s are cached, other errors are not.
func (g *GitHub) LookupCommit(ctx context.Context, repoURL, sha string) (Commit, error) {
	owner, repo, ok := ParseGitHubRepo(repoURL)
	if !ok {
		return Commit{}, fmt.Errorf("not a github.com repo URL")
	}
	sha = strings.TrimSpace(sha)
	if sha == "" {
		return Commit{}, fmt.Errorf("empty commit sha")
	}
	key := commitCacheKey(owner, repo, sha)
	if cached, hit := g.cachedCommit(key); hit {
		if !cached.found {
			return Commit{}, fmt.Errorf("github commit not found")
		}
		return cached.commit, nil
	}
	path := fmt.Sprintf("/repos/%s/%s/commits/%s", url.PathEscape(owner), url.PathEscape(repo), url.PathEscape(sha))
	var raw gitCommit
	if _, err := g.getJSON(ctx, path, &raw); err != nil {
		var apiErr *apiError
		if errors.As(err, &apiErr) && apiErr.Status == http.StatusNotFound {
			g.storeCommit(key, cachedCommit{})
		}
		return Commit{}, err
	}
	got := commitFromAPI(raw, repoURL, sha)
	g.storeCommit(key, cachedCommit{commit: got, found: true})
	if raw.SHA != "" && !strings.EqualFold(raw.SHA, sha) {
		g.storeCommit(commitCacheKey(owner, repo, raw.SHA), cachedCommit{commit: got, found: true})
	}
	return got, nil
}

// ListWorkflowRuns returns recent Actions runs for a github.com repoURL.
// In-progress and queued runs are listed first; the rest keep GitHub's
// newest-first order. Results are cached briefly so the console poll
// does not hammer GitHub.
func (g *GitHub) ListWorkflowRuns(ctx context.Context, repoURL string, limit int) ([]WorkflowRun, error) {
	owner, repo, ok := ParseGitHubRepo(repoURL)
	if !ok {
		return nil, fmt.Errorf("not a github.com repo URL")
	}
	if limit <= 0 {
		limit = defaultWorkflowRuns
	}
	if limit > maxWorkflowRuns {
		limit = maxWorkflowRuns
	}
	key := workflowCacheKey(owner, repo, limit)
	if cached, hit := g.cachedRuns(key); hit {
		return slices.Clone(cached.runs), cached.err
	}
	path := fmt.Sprintf("/repos/%s/%s/actions/runs?per_page=%d",
		url.PathEscape(owner), url.PathEscape(repo), limit)
	var raw workflowRunsAPI
	if _, err := g.getJSON(ctx, path, &raw); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		err = authHint(err, "token needs actions:read for private workflow runs")
		g.storeRuns(key, cachedRuns{err: err, at: time.Now()})
		return nil, err
	}
	out := make([]WorkflowRun, 0, len(raw.WorkflowRuns))
	for _, run := range raw.WorkflowRuns {
		out = append(out, workflowFromAPI(run, repoURL))
	}
	slices.SortStableFunc(out, cmpWorkflowRun)
	g.storeRuns(key, cachedRuns{runs: slices.Clone(out), at: time.Now()})
	return out, nil
}

func commitFromAPI(raw gitCommit, repoURL, sha string) Commit {
	out := Commit{SHA: cmp.Or(raw.SHA, sha)}
	out.Message = firstLine(raw.Commit.Message)
	out.Author = cmp.Or(raw.Commit.Author.Name, raw.Author.Login)
	out.URL = cmp.Or(raw.HTMLURL, GitHubCommitURL(repoURL, out.SHA))
	return out
}

func workflowFromAPI(raw workflowRunAPI, repoURL string) WorkflowRun {
	title := firstLine(raw.DisplayTitle)
	if title == "" {
		title = firstLine(raw.HeadCommit.Message)
	}
	out := WorkflowRun{
		ID:        raw.ID,
		Name:      raw.Name,
		Title:     title,
		Number:    raw.RunNumber,
		Event:     strings.ReplaceAll(raw.Event, "_", " "),
		Status:    workflowStatus(raw.Status, raw.Conclusion),
		Branch:    raw.HeadBranch,
		SHA:       raw.HeadSHA,
		Actor:     raw.Actor.Login,
		URL:       raw.HTMLURL,
		CommitURL: GitHubCommitURL(repoURL, raw.HeadSHA),
	}
	started := raw.RunStartedAt
	if started.IsZero() {
		started = raw.CreatedAt
	}
	if !started.IsZero() {
		t := started.UTC()
		out.StartedAt = &t
	}
	return out
}

func cmpWorkflowRun(a, b WorkflowRun) int {
	aAct, bAct := isActiveWorkflow(a.Status), isActiveWorkflow(b.Status)
	switch {
	case aAct == bAct:
		return 0
	case aAct:
		return -1
	default:
		return 1
	}
}

func isActiveWorkflow(status string) bool {
	switch status {
	case "in progress", "queued", "waiting", "pending", "requested":
		return true
	default:
		return false
	}
}

func workflowStatus(status, conclusion string) string {
	status = strings.ToLower(strings.TrimSpace(status))
	conclusion = strings.ToLower(strings.TrimSpace(conclusion))
	if status != "" && status != "completed" {
		return strings.ReplaceAll(status, "_", " ")
	}
	if conclusion != "" {
		return strings.ReplaceAll(conclusion, "_", " ")
	}
	return cmp.Or(strings.ReplaceAll(status, "_", " "), "unknown")
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	return s
}

func authHint(err error, hint string) error {
	var apiErr *apiError
	if hint == "" || !errors.As(err, &apiErr) {
		return err
	}
	if apiErr.Status != http.StatusForbidden && apiErr.Status != http.StatusUnauthorized {
		return err
	}
	return fmt.Errorf("%w (%s)", err, hint)
}

func commitCacheKey(owner, repo, sha string) string {
	return strings.ToLower(owner + "/" + repo + "@" + sha)
}

func (g *GitHub) cachedCommit(key string) (cachedCommit, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.commits == nil {
		return cachedCommit{}, false
	}
	c, ok := g.commits[key]
	return c, ok
}

func (g *GitHub) storeCommit(key string, c cachedCommit) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.commits == nil {
		g.commits = map[string]cachedCommit{}
	}
	g.commits[key] = c
}

func workflowCacheKey(owner, repo string, limit int) string {
	return strings.ToLower(owner+"/"+repo) + "#" + strconv.Itoa(limit)
}

func (g *GitHub) cachedRuns(key string) (cachedRuns, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.runs == nil {
		return cachedRuns{}, false
	}
	c, ok := g.runs[key]
	if !ok || time.Since(c.at) > workflowRunsTTL {
		return cachedRuns{}, false
	}
	return c, true
}

func (g *GitHub) storeRuns(key string, c cachedRuns) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.runs == nil {
		g.runs = map[string]cachedRuns{}
	}
	g.runs[key] = c
}

func sortVersions(out []Version) {
	slices.SortStableFunc(out, func(a, b Version) int {
		if c := b.CreatedAt.Compare(a.CreatedAt); c != 0 {
			return c
		}
		return strings.Compare(a.Tag, b.Tag)
	})
}

type packageVersion struct {
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	Metadata  struct {
		Container struct {
			Tags []string `json:"tags"`
		} `json:"container"`
	} `json:"metadata"`
}

type gitCommit struct {
	SHA     string `json:"sha"`
	HTMLURL string `json:"html_url"`
	Commit  struct {
		Message string `json:"message"`
		Author  struct {
			Name string    `json:"name"`
			Date time.Time `json:"date"`
		} `json:"author"`
		Committer struct {
			Date time.Time `json:"date"`
		} `json:"committer"`
	} `json:"commit"`
	Author struct {
		Login string `json:"login"`
	} `json:"author"`
}

type workflowRunsAPI struct {
	WorkflowRuns []workflowRunAPI `json:"workflow_runs"`
}

type workflowRunAPI struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"`
	DisplayTitle string    `json:"display_title"`
	RunNumber    int       `json:"run_number"`
	Event        string    `json:"event"`
	Status       string    `json:"status"`
	Conclusion   string    `json:"conclusion"`
	HTMLURL      string    `json:"html_url"`
	HeadBranch   string    `json:"head_branch"`
	HeadSHA      string    `json:"head_sha"`
	CreatedAt    time.Time `json:"created_at"`
	RunStartedAt time.Time `json:"run_started_at"`
	Actor        struct {
		Login string `json:"login"`
	} `json:"actor"`
	HeadCommit struct {
		Message string `json:"message"`
	} `json:"head_commit"`
}

func versionFromPackage(repository string, v packageVersion) (Version, bool) {
	tags := v.Metadata.Container.Tags
	if len(tags) == 0 {
		return Version{}, false
	}
	tag := PreferredTag(tags)
	digest := v.Name
	if digest != "" && !strings.HasPrefix(digest, "sha256:") {
		digest = "sha256:" + digest
	}
	ref := Ref{Repository: repository, Tag: tag, Digest: digest}
	copied := slices.Clone(tags)
	return Version{
		Repository: repository,
		Ref:        ref.String(),
		Tag:        tag,
		Digest:     digest,
		Tags:       copied,
		CreatedAt:  v.CreatedAt,
	}, true
}

type apiError struct {
	Status int
	Body   string
}

func (e *apiError) Error() string {
	return fmt.Sprintf("github %d: %s", e.Status, e.Body)
}

func (g *GitHub) getJSON(ctx context.Context, path string, dest any) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.url(path), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", githubAPIVersion)
	req.Header.Set("User-Agent", "deploybot")
	if g.Token != "" {
		req.Header.Set("Authorization", "Bearer "+g.Token)
	}
	resp, err := logx.Do("github", g.client(), req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(body))
		var ge struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(body, &ge) == nil && ge.Message != "" {
			msg = ge.Message
		}
		return "", &apiError{Status: resp.StatusCode, Body: msg}
	}
	if err := json.Unmarshal(body, dest); err != nil {
		return "", fmt.Errorf("decode github json: %w", err)
	}
	return resp.Header.Get("Link"), nil
}

func (g *GitHub) url(path string) string {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	base := g.APIBase
	if base == "" {
		base = "https://api.github.com"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return strings.TrimRight(base, "/") + path
}

func (g *GitHub) client() *http.Client {
	if g.HTTPClient != nil {
		return g.HTTPClient
	}
	return http.DefaultClient
}

func parseNextLink(h string) string {
	for _, part := range strings.Split(h, ",") {
		part = strings.TrimSpace(part)
		if !strings.Contains(part, `rel="next"`) {
			continue
		}
		start := strings.Index(part, "<")
		end := strings.Index(part, ">")
		if start >= 0 && end > start {
			return part[start+1 : end]
		}
	}
	return ""
}
