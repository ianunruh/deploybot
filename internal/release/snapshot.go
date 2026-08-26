package release

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/ianunruh/deploybot/internal/argo"
	"github.com/ianunruh/deploybot/internal/kube"
	"github.com/ianunruh/deploybot/internal/valkey"
)

const liveKeyPrefix = "deploybot:live:"

// clusterSnap is one cluster's live Argo + workload state. Request-path
// reads use this; WatchLive fills it off the request path.
type clusterSnap struct {
	UpdatedAt time.Time                `json:"updatedAt"`
	CheckedAt time.Time                `json:"checkedAt"`
	Connected bool                     `json:"connected"`
	Message   string                   `json:"message,omitempty"`
	Apps      map[string]argo.Status   `json:"apps"`
	Workloads map[string]kube.Workload `json:"workloads"`
}

type liveStore struct {
	mu      sync.RWMutex
	items   map[string]clusterSnap
	persist *valkey.Client
}

func newLiveStore(addr string) *liveStore {
	s := &liveStore{items: map[string]clusterSnap{}}
	if a := strings.TrimSpace(addr); a != "" {
		s.persist = &valkey.Client{Addr: a}
	}
	return s
}

func (s *Service) initLive() *liveStore {
	s.initCaches()
	return s.live
}

func (s *Service) liveSnapshot(cluster string) (clusterSnap, bool) {
	if s == nil || s.live == nil || cluster == "" {
		return clusterSnap{}, false
	}
	return s.live.get(cluster)
}

func (c *liveStore) get(cluster string) (clusterSnap, bool) {
	if c == nil || cluster == "" {
		return clusterSnap{}, false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	snap, ok := c.items[cluster]
	if !ok {
		return clusterSnap{}, false
	}
	return cloneClusterSnap(snap), true
}

func (c *liveStore) put(cluster string, snap clusterSnap) {
	if c == nil || cluster == "" {
		return
	}
	cloned := cloneClusterSnap(snap)
	c.mu.Lock()
	if c.items == nil {
		c.items = map[string]clusterSnap{}
	}
	c.items[cluster] = cloned
	persist := c.persist
	c.mu.Unlock()
	if persist == nil {
		return
	}
	raw, err := json.Marshal(cloned)
	if err != nil {
		slog.Warn("live persist marshal", "cluster", cluster, "err", err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := persist.Set(ctx, liveKeyPrefix+cluster, raw); err != nil {
		slog.Warn("live persist", "cluster", cluster, "err", err)
	}
}

func (c *liveStore) hydrate(ctx context.Context, clusters []string) {
	if c == nil || c.persist == nil {
		return
	}
	for _, name := range clusters {
		if ctx.Err() != nil {
			return
		}
		raw, err := c.persist.Get(ctx, liveKeyPrefix+name)
		if err != nil {
			slog.Warn("live hydrate", "cluster", name, "err", err)
			continue
		}
		if len(raw) == 0 {
			continue
		}
		var snap clusterSnap
		if err := json.Unmarshal(raw, &snap); err != nil {
			slog.Warn("live hydrate decode", "cluster", name, "err", err)
			continue
		}
		c.mu.Lock()
		if c.items == nil {
			c.items = map[string]clusterSnap{}
		}
		c.items[name] = cloneClusterSnap(snap)
		c.mu.Unlock()
	}
}

func cloneClusterSnap(in clusterSnap) clusterSnap {
	out := in
	if !in.UpdatedAt.IsZero() {
		out.UpdatedAt = in.UpdatedAt.UTC()
	}
	if !in.CheckedAt.IsZero() {
		out.CheckedAt = in.CheckedAt.UTC()
	}
	out.Apps = cloneApps(in.Apps)
	if in.Workloads == nil {
		return out
	}
	out.Workloads = make(map[string]kube.Workload, len(in.Workloads))
	for k, w := range in.Workloads {
		out.Workloads[k] = cloneWorkload(w)
	}
	return out
}

func cloneApps(in map[string]argo.Status) map[string]argo.Status {
	if in == nil {
		return nil
	}
	out := make(map[string]argo.Status, len(in))
	for k, st := range in {
		if st.DeployedAt != nil {
			t := st.DeployedAt.UTC()
			st.DeployedAt = &t
		}
		out[k] = st
	}
	return out
}

func cloneWorkload(w kube.Workload) kube.Workload {
	if len(w.Pods) == 0 {
		return w
	}
	pods := make([]kube.Pod, len(w.Pods))
	copy(pods, w.Pods)
	for i := range pods {
		if pods[i].CreatedAt != nil {
			t := pods[i].CreatedAt.UTC()
			pods[i].CreatedAt = &t
		}
		if pods[i].RestartedAt != nil {
			t := pods[i].RestartedAt.UTC()
			pods[i].RestartedAt = &t
		}
	}
	w.Pods = pods
	return w
}

func workloadKey(namespace, kind, name string) string {
	if kind == "" {
		kind = "Deployment"
	}
	return namespace + "/" + kind + "/" + name
}
