package release

import (
	"context"
	"sync"
	"time"

	"github.com/ianunruh/deploybot/internal/argo"
	"github.com/ianunruh/deploybot/internal/render"
)

const statusArgoTimeout = 4 * time.Second

type StageStatus struct {
	Name        string     `json:"name"`
	Hostname    string     `json:"hostname"`
	Image       string     `json:"image,omitempty"`
	Sync        string     `json:"sync"`
	Health      string     `json:"health"`
	Revision    string     `json:"revision,omitempty"`
	Message     string     `json:"message,omitempty"`
	DeployedAt  *time.Time `json:"deployedAt,omitempty"`
	ArgoURL     string     `json:"argoURL,omitempty"`
	HeadlampURL string     `json:"headlampURL,omitempty"`
	GrafanaURL  string     `json:"grafanaURL,omitempty"`
	LogsURL     string     `json:"logsURL,omitempty"`
}

type Status struct {
	Name       string        `json:"name"`
	Namespace  string        `json:"namespace"`
	ImageRepo  string        `json:"imageRepo"`
	RepoURL    string        `json:"repoURL,omitempty"`
	ProjectURL string        `json:"projectURL,omitempty"`
	Stages     []StageStatus `json:"stages"`
	Apply      bool          `json:"apply"`
	Push       bool          `json:"push"`
	Sync       bool          `json:"sync"`
}

func (s *Service) Status(ctx context.Context, name string) (Status, error) {
	d, err := s.Catalog.Get(name)
	if err != nil {
		return Status{}, err
	}
	tree, err := s.workingTree(ctx, d)
	if err != nil {
		return Status{}, err
	}
	out := Status{
		Name:       d.Metadata.Name,
		Namespace:  d.Spec.Namespace,
		ImageRepo:  d.Spec.Image.Repository,
		RepoURL:    d.Spec.Links.RepoURL,
		ProjectURL: d.Spec.Links.ProjectURL,
		Apply:      s.Apply,
		Push:       s.Push,
		Sync:       s.Sync,
	}
	out.Stages = make([]StageStatus, len(d.Spec.Stages))
	var wg sync.WaitGroup
	for i, st := range d.Spec.Stages {
		ss := StageStatus{
			Name:     st.Name,
			Hostname: st.Hostname,
			Sync:     "unknown",
			Health:   "unknown",
		}
		ss.HeadlampURL, ss.GrafanaURL, ss.LogsURL = ObservabilityURLs(st.Name, d.Spec.Namespace)
		if img, err := render.CurrentImage(tree, d, st.Name); err == nil {
			ss.Image = img.Compact()
		}
		if s.Argo != nil {
			if c := s.Argo.ForStage(st.Name); c != nil {
				ss.ArgoURL = argo.AppURL(c, d.Spec.Argo.Name)
				out.Stages[i] = ss
				wg.Add(1)
				go func(i int, c argo.Client) {
					defer wg.Done()
					gctx, cancel := context.WithTimeout(ctx, statusArgoTimeout)
					defer cancel()
					got, err := c.Get(gctx, d.Spec.Argo.Name)
					if err != nil {
						out.Stages[i].Message = err.Error()
						return
					}
					out.Stages[i].Health = got.Health
					out.Stages[i].Sync = got.Sync
					out.Stages[i].Revision = got.Revision
					out.Stages[i].Message = got.Message
					out.Stages[i].DeployedAt = got.DeployedAt
				}(i, c)
				continue
			}
		}
		out.Stages[i] = ss
	}
	wg.Wait()
	return out, nil
}

// LatestDeployedAt returns the newest Argo history deployedAt across stages
// for each catalog deployable. Missing Argo clients and Get errors are skipped.
func (s *Service) LatestDeployedAt(ctx context.Context) map[string]*time.Time {
	out := map[string]*time.Time{}
	if s == nil || s.Argo == nil || s.Catalog == nil {
		return out
	}
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, d := range s.Catalog.List() {
		name := d.Metadata.Name
		app := d.Spec.Argo.Name
		for _, stage := range d.StageNames() {
			c := s.Argo.ForStage(stage)
			if c == nil {
				continue
			}
			wg.Add(1)
			go func(name, app string, c argo.Client) {
				defer wg.Done()
				gctx, cancel := context.WithTimeout(ctx, statusArgoTimeout)
				defer cancel()
				got, err := c.Get(gctx, app)
				if err != nil || got.DeployedAt == nil {
					return
				}
				mu.Lock()
				defer mu.Unlock()
				if prev := out[name]; prev == nil || got.DeployedAt.After(*prev) {
					out[name] = got.DeployedAt
				}
			}(name, app, c)
		}
	}
	wg.Wait()
	return out
}
