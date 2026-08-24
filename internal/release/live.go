package release

import (
	"context"
	"sync"
	"time"

	"github.com/ianunruh/deploybot/internal/argo"
	"github.com/ianunruh/deploybot/internal/kube"
	"github.com/ianunruh/deploybot/internal/spec"
)

const statusKubeTimeout = 4 * time.Second

// StageWorkload is live Deployment/StatefulSet state for one stage.
type StageWorkload struct {
	Name     string         `json:"name"`
	Workload *kube.Workload `json:"workload,omitempty"`
}

// LiveWorkloads is per-stage kube replica/pod state for the detail page.
type LiveWorkloads struct {
	Stages []StageWorkload `json:"stages"`
}

// LiveWorkloads reads Deployment/StatefulSet replica counts and pods.
// Status, Latest, and WatchFlows skip this; only the detail overview fetches it.
func (s *Service) LiveWorkloads(ctx context.Context, name string) (LiveWorkloads, error) {
	d, err := s.Catalog.Get(name)
	if err != nil {
		return LiveWorkloads{}, err
	}
	stages := make([]StageStatus, len(d.Spec.Stages))
	for i, st := range d.Spec.Stages {
		stages[i] = StageStatus{Name: st.Name}
	}
	s.attachLive(ctx, d, stages)
	out := LiveWorkloads{Stages: make([]StageWorkload, len(stages))}
	for i, st := range stages {
		out.Stages[i] = StageWorkload{Name: st.Name, Workload: st.Workload}
	}
	return out, nil
}

// attachLive fills per-stage Deployment/StatefulSet replica counts and pods.
func (s *Service) attachLive(ctx context.Context, d *spec.Deployable, stages []StageStatus) {
	if s == nil || s.Argo == nil || d == nil || len(stages) == 0 {
		return
	}
	var wg sync.WaitGroup
	for i, st := range d.Spec.Stages {
		if i >= len(stages) {
			break
		}
		rest := argo.REST(s.Argo.ForStage(st.Name))
		if rest == nil {
			continue
		}
		wg.Go(func() {
			gctx, cancel := context.WithTimeout(ctx, statusKubeTimeout)
			defer cancel()
			live := kube.ReadWorkload(gctx, rest, d.Spec.Namespace, d.Spec.Workload.Kind, d.Metadata.Name)
			stages[i].Workload = &live
		})
	}
	wg.Wait()
}
