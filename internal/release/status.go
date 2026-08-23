package release

import (
	"context"

	"github.com/ianunruh/deploybot/internal/argo"
	"github.com/ianunruh/deploybot/internal/render"
)

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
	for _, st := range d.Spec.Stages {
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
				got, err := c.Get(ctx, d.Spec.Argo.Name)
				if err != nil {
					ss.Message = err.Error()
				} else {
					ss.Health = got.Health
					ss.Sync = got.Sync
					ss.Revision = got.Revision
					ss.Message = got.Message
				}
			}
		}
		out.Stages = append(out.Stages, ss)
	}
	return out, nil
}
