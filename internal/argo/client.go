package argo

import (
	"bytes"
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

type Status struct {
	Name     string
	Health   string
	Sync     string
	Revision string
	Message  string
}

func (s Status) Healthy() bool {
	return strings.EqualFold(s.Health, "Healthy")
}

func (s Status) Degraded() bool {
	switch strings.ToLower(s.Health) {
	case "degraded", "missing":
		return true
	default:
		return false
	}
}

type Client interface {
	Get(ctx context.Context, app string) (Status, error)
	Sync(ctx context.Context, app string, prune bool) error
}

type HTTPClient struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

func (c *HTTPClient) client() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return http.DefaultClient
}

func (c *HTTPClient) Get(ctx context.Context, app string) (Status, error) {
	u, err := url.JoinPath(c.BaseURL, "/api/v1/applications", app)
	if err != nil {
		return Status{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return Status{}, err
	}
	c.auth(req)
	resp, err := c.client().Do(req)
	if err != nil {
		return Status{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Status{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return Status{}, fmt.Errorf("argo get %s: %s: %s", app, resp.Status, body)
	}
	var raw argoApp
	if err := json.Unmarshal(body, &raw); err != nil {
		return Status{}, fmt.Errorf("decode argo app: %w", err)
	}
	return Status{
		Name:     raw.Metadata.Name,
		Health:   raw.Status.Health.Status,
		Sync:     raw.Status.Sync.Status,
		Revision: raw.Status.Sync.Revision,
		Message:  raw.Status.Health.Message,
	}, nil
}

func (c *HTTPClient) Sync(ctx context.Context, app string, prune bool) error {
	u, err := url.JoinPath(c.BaseURL, "/api/v1/applications", app, "sync")
	if err != nil {
		return err
	}
	payload, err := json.Marshal(map[string]any{"prune": prune})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	c.auth(req)
	resp, err := c.client().Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("argo sync %s: %s: %s", app, resp.Status, body)
	}
	return nil
}

func (c *HTTPClient) auth(req *http.Request) {
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
}

type argoApp struct {
	Metadata struct {
		Name string `json:"name"`
	} `json:"metadata"`
	Status struct {
		Health struct {
			Status  string `json:"status"`
			Message string `json:"message"`
		} `json:"health"`
		Sync struct {
			Status   string `json:"status"`
			Revision string `json:"revision"`
		} `json:"sync"`
	} `json:"status"`
}

func WaitHealthy(ctx context.Context, c Client, app string, poll time.Duration) error {
	if poll <= 0 {
		poll = 2 * time.Second
	}
	ticker := time.NewTicker(poll)
	defer ticker.Stop()
	var last Status
	for {
		st, err := c.Get(ctx, app)
		if err != nil {
			return err
		}
		last = st
		if st.Healthy() {
			return nil
		}
		if st.Degraded() {
			return fmt.Errorf("app %s %s: %s", app, st.Health, st.Message)
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("wait for %s healthy (last %s/%s): %w", app, last.Health, last.Sync, ctx.Err())
		case <-ticker.C:
		}
	}
}

// Router returns an Argo client for a promotion stage.
type Router interface {
	ForStage(stage string) Client
}

// Endpoints maps stage name -> Argo API.
type Endpoints map[string]HTTPClient

func EndpointsFromEnv() Endpoints {
	out := Endpoints{}
	if base := os.Getenv("DEPLOYBOT_ARGO_URL"); base != "" {
		def := HTTPClient{BaseURL: base, Token: os.Getenv("DEPLOYBOT_ARGO_TOKEN")}
		out[""] = def
	}
	for _, e := range os.Environ() {
		key, val, ok := strings.Cut(e, "=")
		if !ok || val == "" {
			continue
		}
		if name, ok := strings.CutPrefix(key, "DEPLOYBOT_ARGO_URL_"); ok {
			stage := strings.ToLower(name)
			ep := out[stage]
			ep.BaseURL = val
			out[stage] = ep
		}
		if name, ok := strings.CutPrefix(key, "DEPLOYBOT_ARGO_TOKEN_"); ok {
			stage := strings.ToLower(name)
			ep := out[stage]
			ep.Token = val
			out[stage] = ep
		}
	}
	return out
}

func (e Endpoints) ForStage(stage string) Client {
	if c, ok := e[strings.ToLower(stage)]; ok && c.BaseURL != "" {
		cp := c
		return &cp
	}
	if c, ok := e[""]; ok && c.BaseURL != "" {
		cp := c
		return &cp
	}
	return nil
}
