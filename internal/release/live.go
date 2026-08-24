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

// attachLive fills per-stage Deployment/StatefulSet replica counts and pods.
// Catalog Latest skips this; the detail page is the live view.
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
