package image

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	maxHubPages  = 3
	hubPageSize  = 100
	hubUserAgent = "deploybot"
)

// DockerHub lists public tags through the Docker Hub HTTP API.
type DockerHub struct {
	Token      string
	APIBase    string
	HTTPClient *http.Client
}

func NewDockerHub() *DockerHub {
	return &DockerHub{
		Token:      strings.TrimSpace(os.Getenv("DEPLOYBOT_DOCKERHUB_TOKEN")),
		APIBase:    "https://hub.docker.com",
		HTTPClient: &http.Client{Timeout: 20 * time.Second},
	}
}

func (h *DockerHub) List(ctx context.Context, repository, _ string) (Listing, error) {
	ns, name, err := parseDockerHub(repository)
	if err != nil {
		return Listing{}, err
	}
	repo := CanonicalRepository(repository)
	// Hub treats ordering=last_updated as newest first. The leading-minus form
	// returns oldest first and is what the public docs suggest — do not use it.
	path := fmt.Sprintf("/v2/namespaces/%s/repositories/%s/tags?page_size=%d&ordering=last_updated",
		url.PathEscape(ns), url.PathEscape(name), hubPageSize)

	var out []Version
	for range maxHubPages {
		if path == "" || len(out) >= MaxVersions {
			break
		}
		page, next, err := h.listPage(ctx, path)
		if err != nil {
			return Listing{}, err
		}
		for _, tag := range page {
			if len(out) >= MaxVersions {
				break
			}
			ver, ok := versionFromHub(repo, tag)
			if !ok {
				continue
			}
			out = append(out, ver)
		}
		path = next
	}
	if out == nil {
		out = []Version{}
	}
	return Listing{Source: "dockerhub", Versions: out}, nil
}

func parseDockerHub(repository string) (ns, name string, err error) {
	repo := CanonicalRepository(repository)
	host, rest, ok := strings.Cut(repo, "/")
	if !ok || host != "docker.io" {
		return "", "", fmt.Errorf("image listing only supports docker.io, got %q", repository)
	}
	ns, name, ok = strings.Cut(rest, "/")
	if !ok || ns == "" || name == "" || strings.Contains(name, "/") {
		return "", "", fmt.Errorf("invalid docker hub repository %q", repository)
	}
	return ns, name, nil
}

type hubTagPage struct {
	Next    string   `json:"next"`
	Results []hubTag `json:"results"`
}

type hubTag struct {
	Name        string     `json:"name"`
	LastUpdated time.Time  `json:"last_updated"`
	Digest      string     `json:"digest"`
	Images      []hubImage `json:"images"`
}

type hubImage struct {
	Architecture string `json:"architecture"`
	OS           string `json:"os"`
	Digest       string `json:"digest"`
}

func versionFromHub(repository string, t hubTag) (Version, bool) {
	tag := strings.TrimSpace(t.Name)
	if tag == "" || skipArchTag(tag) {
		return Version{}, false
	}
	digest := strings.TrimSpace(hubDigest(t))
	if digest != "" && !strings.HasPrefix(digest, "sha256:") {
		digest = "sha256:" + digest
	}
	ref := Ref{Repository: repository, Tag: tag, Digest: digest}
	return Version{
		Repository: repository,
		Ref:        ref.String(),
		Tag:        tag,
		Digest:     digest,
		Tags:       []string{tag},
		CreatedAt:  t.LastUpdated,
	}, true
}

func hubDigest(t hubTag) string {
	if d := strings.TrimSpace(t.Digest); d != "" {
		return d
	}
	var amd, any string
	for _, im := range t.Images {
		d := strings.TrimSpace(im.Digest)
		if d == "" || im.OS == "unknown" || im.Architecture == "unknown" {
			continue
		}
		if any == "" {
			any = d
		}
		if im.OS == "linux" && im.Architecture == "amd64" {
			amd = d
		}
	}
	return cmp.Or(amd, any)
}

func (h *DockerHub) listPage(ctx context.Context, path string) ([]hubTag, string, error) {
	var page hubTagPage
	if err := h.getJSON(ctx, path, &page); err != nil {
		return nil, "", err
	}
	next := strings.TrimSpace(page.Next)
	if next != "" && !strings.HasPrefix(next, "http://") && !strings.HasPrefix(next, "https://") {
		next = h.url(next)
	}
	return page.Results, next, nil
}

type hubAPIError struct {
	Status int
	Body   string
}

func (e *hubAPIError) Error() string {
	return fmt.Sprintf("docker hub %d: %s", e.Status, e.Body)
}

func (h *DockerHub) getJSON(ctx context.Context, path string, dest any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.url(path), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", hubUserAgent)
	if h.Token != "" {
		req.Header.Set("Authorization", "Bearer "+h.Token)
	}
	resp, err := h.client().Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(body))
		var he struct {
			Message string `json:"message"`
			Detail  string `json:"detail"`
			Err     string `json:"error"`
		}
		if json.Unmarshal(body, &he) == nil {
			msg = firstNonEmpty(he.Message, he.Detail, he.Err, msg)
		}
		return &hubAPIError{Status: resp.StatusCode, Body: msg}
	}
	if err := json.Unmarshal(body, dest); err != nil {
		return fmt.Errorf("decode docker hub json: %w", err)
	}
	return nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func (h *DockerHub) url(path string) string {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	base := h.APIBase
	if base == "" {
		base = "https://hub.docker.com"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return strings.TrimRight(base, "/") + path
}

func (h *DockerHub) client() *http.Client {
	if h.HTTPClient != nil {
		return h.HTTPClient
	}
	return http.DefaultClient
}
