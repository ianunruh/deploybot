package release

import (
	"context"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/ianunruh/deploybot/internal/argo"
	"github.com/ianunruh/deploybot/internal/kube"
)

const (
	defaultLiveEvery  = 5 * time.Second
	statusKubeTimeout = 4 * time.Second
)

// WatchLive polls each configured cluster independently and writes Argo
// apps + workloads into the live snapshot. API reads use that snapshot
// and never wait on kube. Optional Valkey is hydrate + persist only.
func (s *Service) WatchLive(ctx context.Context) {
	if s == nil {
		return
	}
	s.initLive()
	names := s.liveClusterNames()
	if s.live != nil {
		s.live.hydrate(ctx, names)
	}
	every := s.LiveEvery
	if every <= 0 {
		every = defaultLiveEvery
	}
	var wg sync.WaitGroup
	for _, name := range names {
		wg.Go(func() { s.watchCluster(ctx, name, every) })
	}
	wg.Wait()
}

func (s *Service) watchCluster(ctx context.Context, name string, every time.Duration) {
	s.refreshCluster(ctx, name)
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.refreshCluster(ctx, name)
		}
	}
}

// RefreshLive fills every cluster snapshot once. Tests use it instead of WatchLive.
func (s *Service) RefreshLive(ctx context.Context) {
	if s == nil {
		return
	}
	s.initLive()
	for _, name := range s.liveClusterNames() {
		s.refreshCluster(ctx, name)
	}
}

func (s *Service) refreshCluster(ctx context.Context, name string) {
	if s == nil || name == "" || ctx.Err() != nil {
		return
	}
	live := s.initLive()
	prev, _ := live.get(name)
	now := time.Now().UTC()
	snap := prev
	snap.CheckedAt = now
	if snap.Apps == nil {
		snap.Apps = map[string]argo.Status{}
	}
	if snap.Workloads == nil {
		snap.Workloads = map[string]kube.Workload{}
	}

	if s.Argo == nil {
		snap.Connected = false
		snap.Message = "no Argo endpoint"
		live.put(name, snap)
		return
	}
	c := s.Argo.ForStage(name)
	if c == nil {
		snap.Connected = false
		snap.Message = "no Argo endpoint"
		live.put(name, snap)
		return
	}

	gctx, cancel := context.WithTimeout(ctx, statusArgoTimeout)
	apps, err := listApps(gctx, c)
	cancel()
	if err != nil {
		snap.Connected = false
		snap.Message = err.Error()
		live.put(name, snap)
		slog.Warn("live cluster", "cluster", name, "err", err)
		return
	}

	workloads := s.pollWorkloads(ctx, name, argo.REST(c))
	snap.Apps = apps
	snap.Workloads = workloads
	snap.Connected = true
	snap.Message = ""
	snap.UpdatedAt = now
	live.put(name, snap)
}

func (s *Service) pollWorkloads(ctx context.Context, stage string, rest *kube.REST) map[string]kube.Workload {
	out := map[string]kube.Workload{}
	if s == nil || s.Catalog == nil || rest == nil {
		return out
	}
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, d := range s.Catalog.List() {
		has := false
		for _, st := range d.Spec.Stages {
			if st.Name == stage {
				has = true
				break
			}
		}
		if !has {
			continue
		}
		wg.Go(func() {
			gctx, cancel := context.WithTimeout(ctx, statusKubeTimeout)
			defer cancel()
			live := kube.ReadWorkload(gctx, rest, d.Spec.Namespace, d.Spec.Workload.Kind, d.Metadata.Name)
			key := workloadKey(d.Spec.Namespace, d.Spec.Workload.Kind, d.Metadata.Name)
			mu.Lock()
			out[key] = live
			mu.Unlock()
		})
	}
	wg.Wait()
	return out
}

func (s *Service) liveClusterNames() []string {
	seen := map[string]struct{}{}
	if s != nil && s.Clusters != nil {
		for name := range s.Clusters {
			if name != "" {
				seen[name] = struct{}{}
			}
		}
	}
	if s != nil && s.Catalog != nil {
		for _, d := range s.Catalog.List() {
			for _, st := range d.Spec.Stages {
				if st.Name != "" {
					seen[st.Name] = struct{}{}
				}
			}
		}
	}
	out := make([]string, 0, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	slices.Sort(out)
	return out
}

func listApps(ctx context.Context, c argo.Client) (map[string]argo.Status, error) {
	items, err := c.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[string]argo.Status, len(items))
	for _, st := range items {
		if st.Name == "" {
			continue
		}
		out[st.Name] = st
	}
	return out, nil
}
