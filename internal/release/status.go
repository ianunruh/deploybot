package release

import (
	"context"
	"sync"
	"time"

	"github.com/ianunruh/deploybot/internal/argo"
	"github.com/ianunruh/deploybot/internal/image"
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
	PinnedAt    *time.Time `json:"pinnedAt,omitempty"`
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
	Flow       Flow          `json:"flow"`
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
	refs := make([]image.Ref, len(d.Spec.Stages))
	events, err := s.overlayChanges(ctx, d, defaultHistoryLimit)
	if err != nil {
		events = nil
	}
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
			refs[i] = img
			ss.PinnedAt = pinTime(events, st.Name, img)
		}
		if s.Argo != nil {
			if c := s.Argo.ForStage(st.Name); c != nil {
				ss.ArgoURL = argo.AppURL(c, d.Spec.Argo.Name)
				out.Stages[i] = ss
				wg.Go(func() {
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
				})
				continue
			}
		}
		out.Stages[i] = ss
	}
	wg.Wait()
	snaps := make([]stageSnap, len(d.Spec.Stages))
	for i, st := range d.Spec.Stages {
		snaps[i] = stageSnap{
			name:     st.Name,
			ref:      refs[i],
			health:   out.Stages[i].Health,
			pinnedAt: out.Stages[i].PinnedAt,
			policy:   st.Promote,
			hasArgo:  s.Argo != nil && s.Argo.ForStage(st.Name) != nil,
		}
	}
	out.Flow = buildFlow(snaps, time.Now().UTC())
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
			wg.Go(func() {
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
			})
		}
	}
	wg.Wait()
	return out
}
