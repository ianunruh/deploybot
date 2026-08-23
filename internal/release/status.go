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
	Name        string `json:"name"`
	Hostname    string `json:"hostname"`
	Image       string `json:"image,omitempty"`
	Sync        string `json:"sync"`
	Health      string `json:"health"`
	Revision    string `json:"revision,omitempty"`
	Message     string `json:"message,omitempty"`
	ArgoURL     string `json:"argoURL,omitempty"`
	HeadlampURL string `json:"headlampURL,omitempty"`
	GrafanaURL  string `json:"grafanaURL,omitempty"`
	LogsURL     string `json:"logsURL,omitempty"`
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
				}(i, c)
				continue
			}
		}
		out.Stages[i] = ss
	}
	wg.Wait()
	return out, nil
}
