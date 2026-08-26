package release

import (
	"context"

	"github.com/ianunruh/deploybot/internal/kube"
	"github.com/ianunruh/deploybot/internal/spec"
)

// StageWorkload is live Deployment/StatefulSet state for one stage.
type StageWorkload struct {
	Name     string         `json:"name"`
	Workload *kube.Workload `json:"workload,omitempty"`
}

// LiveWorkloads is per-stage kube replica/pod state for the detail page.
type LiveWorkloads struct {
	Stages []StageWorkload `json:"stages"`
}

// LiveWorkloads is Deployment/StatefulSet replica counts and pods from the
// WatchLive snapshot. Status, Latest, and WatchFlows skip this; only the
// detail overview fetches it.
func (s *Service) LiveWorkloads(ctx context.Context, name string) (LiveWorkloads, error) {
	d, err := s.Catalog.Get(name)
	if err != nil {
		return LiveWorkloads{}, err
	}
	stages := make([]StageStatus, len(d.Spec.Stages))
	for i, st := range d.Spec.Stages {
		stages[i] = StageStatus{Name: st.Name}
	}
	s.attachLive(d, stages)
	out := LiveWorkloads{Stages: make([]StageWorkload, len(stages))}
	for i, st := range stages {
		out.Stages[i] = StageWorkload{Name: st.Name, Workload: st.Workload}
	}
	return out, nil
}

// attachLive fills per-stage Deployment/StatefulSet replica counts and pods
// from the WatchLive snapshot.
func (s *Service) attachLive(d *spec.Deployable, stages []StageStatus) {
	if s == nil || d == nil || len(stages) == 0 {
		return
	}
	for i, st := range d.Spec.Stages {
		if i >= len(stages) {
			break
		}
		key := workloadKey(d.Spec.Namespace, d.Spec.Workload.Kind, d.Metadata.Name)
		if snap, ok := s.liveSnapshot(st.Name); ok {
			wl := snap.Workloads[key]
			stages[i].Workload = &wl
			continue
		}
		stages[i].Workload = &kube.Workload{
			Kind:    d.Spec.Workload.Kind,
			Name:    d.Metadata.Name,
			Message: "waiting for live snapshot",
		}
	}
}
