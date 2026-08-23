package argo

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/ianunruh/deploybot/internal/config"
)

type Status struct {
	Name       string
	Health     string
	Sync       string
	Revision   string
	Message    string
	DeployedAt *time.Time
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

func joinAppURL(base, app string) string {
	if base == "" || app == "" {
		return ""
	}
	u, err := url.JoinPath(strings.TrimRight(base, "/"), "applications", app)
	if err != nil {
		return ""
	}
	return u
}

// AppURL returns the Argo CD UI page for app when the client knows one.
func AppURL(c Client, app string) string {
	type linker interface {
		AppURL(app string) string
	}
	if l, ok := c.(linker); ok {
		return l.AppURL(app)
	}
	return ""
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
		History []struct {
			DeployedAt time.Time `json:"deployedAt"`
		} `json:"history"`
		OperationState struct {
			FinishedAt *time.Time `json:"finishedAt"`
		} `json:"operationState"`
	} `json:"status"`
}

func statusFrom(raw argoApp) Status {
	return Status{
		Name:       raw.Metadata.Name,
		Health:     raw.Status.Health.Status,
		Sync:       raw.Status.Sync.Status,
		Revision:   raw.Status.Sync.Revision,
		Message:    raw.Status.Health.Message,
		DeployedAt: raw.deployedAt(),
	}
}

// deployedAt is the last recorded Application sync. History is preferred
// (Argo's History/Rollback clock); finishedAt is only used when history is
// empty so a failed or in-flight operation does not hide the last success.
func (a argoApp) deployedAt() *time.Time {
	hist := a.Status.History
	for i := len(hist) - 1; i >= 0; i-- {
		if hist[i].DeployedAt.IsZero() {
			continue
		}
		t := hist[i].DeployedAt.UTC()
		return &t
	}
	if t := a.Status.OperationState.FinishedAt; t != nil && !t.IsZero() {
		u := t.UTC()
		return &u
	}
	return nil
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

// Endpoints maps stage name -> Kubernetes Application client.
type Endpoints map[string]*KubeClient

func EndpointsFromConfig(stages map[string]config.Argo) (Endpoints, error) {
	out := Endpoints{}
	for name, st := range stages {
		name = strings.ToLower(strings.TrimSpace(name))
		if name == "" {
			continue
		}
		ui := strings.TrimRight(st.URL, "/")
		k, err := kubeClientFor(st, name, ui)
		if err != nil {
			return nil, err
		}
		if k != nil {
			out[name] = k
		}
	}
	overlayEnv(out)
	return out, nil
}

func overlayEnv(out Endpoints) {
	if out == nil {
		return
	}
	setURL := func(stage, base string) {
		c := out[stage]
		if c == nil {
			return
		}
		c.UIBaseURL = strings.TrimRight(base, "/")
	}
	if base := os.Getenv("DEPLOYBOT_ARGO_URL"); base != "" {
		for _, c := range out {
			if c != nil && c.UIBaseURL == "" {
				c.UIBaseURL = strings.TrimRight(base, "/")
			}
		}
	}
	for _, e := range os.Environ() {
		key, val, ok := strings.Cut(e, "=")
		if !ok || val == "" {
			continue
		}
		if name, ok := strings.CutPrefix(key, "DEPLOYBOT_ARGO_URL_"); ok {
			setURL(strings.ToLower(name), val)
		}
	}
}

func (e Endpoints) ForStage(stage string) Client {
	if e == nil {
		return nil
	}
	if c := e[strings.ToLower(stage)]; c != nil {
		return c
	}
	return nil
}
