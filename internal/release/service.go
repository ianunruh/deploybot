package release

import (
	"strings"
	"sync"
	"time"

	"github.com/ianunruh/deploybot/internal/argo"
	"github.com/ianunruh/deploybot/internal/catalog"
	"github.com/ianunruh/deploybot/internal/config"
	"github.com/ianunruh/deploybot/internal/gitwrite"
	"github.com/ianunruh/deploybot/internal/image"
)

type Service struct {
	Catalog     *catalog.Catalog
	OpsRepo     string
	Apply       bool
	Push        bool
	Sync        bool
	AutoPin     bool
	Author      gitwrite.Author
	Argo        argo.Router
	Clusters    map[string]config.Cluster
	Wait        time.Duration
	NoWait      bool
	FlowEvery   time.Duration
	LiveEvery   time.Duration
	UpdateEvery time.Duration
	Images      image.Lister
	Commits     image.CommitLookup
	Compares    image.CompareLookup
	Actions     image.WorkflowLookup
	// Valkey is host:port for the local live snapshot. Empty means memory only.
	Valkey string

	// Lock serializes git mutations (HTTP pin, auto-promote, auto-pin).
	Lock      *sync.Mutex
	overlays  *overlayCache
	update    *updateState
	apps      *appsCache
	appsTTL   time.Duration
	live      *liveStore
	cacheOnce *sync.Once
}

type overlayCache struct {
	mu     sync.Mutex
	head   string
	events map[string][]Event
}

type Mutation struct {
	DryRun bool     `json:"dryRun"`
	Commit string   `json:"commit,omitempty"`
	Pushed bool     `json:"pushed"`
	Ref    string   `json:"ref,omitempty"`
	Diff   string   `json:"diff"`
	Files  []string `json:"files"`
	Synced bool     `json:"synced"`
}

// WithSync returns a shallow copy with Argo sync forced on or off for one
// mutation. Sync cannot be turned on if the service has it disabled.
func (s *Service) WithSync(enabled bool) *Service {
	if s == nil || !s.Sync || enabled {
		return s
	}
	s.cachesOnce()
	cp := *s
	cp.Sync = false
	return &cp
}

// WithWait returns a shallow copy that skips the post-sync healthy poll when
// enabled is false. CLI still waits; the console watches live status instead.
func (s *Service) WithWait(enabled bool) *Service {
	if s == nil || enabled {
		return s
	}
	s.cachesOnce()
	cp := *s
	cp.NoWait = true
	return &cp
}

func (s *Service) cachesOnce() *sync.Once {
	if s.cacheOnce == nil {
		s.cacheOnce = new(sync.Once)
	}
	return s.cacheOnce
}

func (s *Service) author() gitwrite.Author {
	if s.Author.Name != "" {
		return s.Author
	}
	return gitwrite.DefaultAuthor()
}

// Cluster returns process-config options for a promotion stage. Stage names
// match cluster keys (homelab, prod). Missing names yield a zero Cluster.
func (s *Service) Cluster(name string) config.Cluster {
	if s == nil || s.Clusters == nil {
		return config.Cluster{}
	}
	return s.Clusters[strings.ToLower(strings.TrimSpace(name))]
}

// Observability is Headlamp / Grafana / logs URLs for a stage namespace.
func (s *Service) Observability(stage, namespace string) (headlamp, grafana, logs string) {
	if s == nil {
		return "", "", ""
	}
	return ObservabilityURLs(s.Cluster(stage), namespace)
}
