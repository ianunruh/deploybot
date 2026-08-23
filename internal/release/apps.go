package release

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ianunruh/deploybot/internal/argo"
)

const defaultAppsTTL = 15 * time.Second

type appsCache struct {
	mu    sync.Mutex
	ttl   time.Duration
	gen   map[string]uint64
	items map[string]cachedApps
	loads map[string]*appsLoad
}

type cachedApps struct {
	at   time.Time
	err  error
	apps map[string]argo.Status
}

type appsLoad struct {
	gen  uint64
	done chan struct{}
	snap cachedApps
}

func newAppsCache(ttl time.Duration) *appsCache {
	if ttl <= 0 {
		ttl = defaultAppsTTL
	}
	return &appsCache{
		ttl:   ttl,
		gen:   map[string]uint64{},
		items: map[string]cachedApps{},
		loads: map[string]*appsLoad{},
	}
}

func (s *Service) initCaches() {
	if s == nil {
		return
	}
	s.cacheOnce.Do(func() {
		if s.apps == nil {
			s.apps = newAppsCache(s.appsTTL)
		}
		if s.update == nil {
			s.update = &updateState{
				ttl:      defaultListingTTL,
				listings: map[string]cachedListing{},
				lastAuto: map[string]time.Time{},
			}
		}
		if s.update.listings == nil {
			s.update.listings = map[string]cachedListing{}
		}
		if s.update.lastAuto == nil {
			s.update.lastAuto = map[string]time.Time{}
		}
		if s.update.ttl <= 0 {
			s.update.ttl = defaultListingTTL
		}
		if s.overlays == nil {
			s.overlays = &overlayCache{events: map[string][]Event{}}
		}
	})
}

func (s *Service) argoApps() *appsCache {
	s.initCaches()
	return s.apps
}

func (s *Service) dropArgo(stage string) {
	if s == nil || s.apps == nil {
		return
	}
	s.apps.drop(stage)
}

func (c *appsCache) drop(stage string) {
	if c == nil || stage == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.gen[stage]++
	delete(c.items, stage)
}

func (c *appsCache) lookup(ctx context.Context, stage, app string, list func(context.Context) (map[string]argo.Status, error)) (argo.Status, error) {
	snap, err := c.snapshot(ctx, stage, list)
	if err != nil {
		return argo.Status{}, err
	}
	st, ok := snap[app]
	if !ok {
		return argo.Status{}, fmt.Errorf("app %s not found", app)
	}
	return st, nil
}

func (c *appsCache) snapshot(ctx context.Context, stage string, list func(context.Context) (map[string]argo.Status, error)) (map[string]argo.Status, error) {
	if c == nil {
		return list(ctx)
	}
	now := time.Now()
	c.mu.Lock()
	if item, ok := c.items[stage]; ok && now.Sub(item.at) < c.ttl {
		c.mu.Unlock()
		if item.err != nil {
			return nil, item.err
		}
		return item.apps, nil
	}
	if load, ok := c.loads[stage]; ok {
		c.mu.Unlock()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-load.done:
			if load.snap.err != nil {
				return nil, load.snap.err
			}
			return load.snap.apps, nil
		}
	}
	load := &appsLoad{gen: c.gen[stage], done: make(chan struct{})}
	c.loads[stage] = load
	c.mu.Unlock()

	apps, err := list(ctx)
	load.snap = cachedApps{at: time.Now(), err: err, apps: cloneApps(apps)}
	canceled := err != nil && ctx.Err() != nil

	c.mu.Lock()
	delete(c.loads, stage)
	if !canceled && load.gen == c.gen[stage] {
		c.items[stage] = load.snap
	}
	c.mu.Unlock()
	close(load.done)

	if err != nil {
		return nil, err
	}
	return load.snap.apps, nil
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
