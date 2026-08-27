package release

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/ianunruh/deploybot/internal/gitwrite"
	"github.com/ianunruh/deploybot/internal/spec"
	"github.com/ianunruh/deploybot/internal/valkey"
)

const (
	overlayKey          = "deploybot:overlays"
	defaultOverlayEvery = 15 * time.Second
	maxOverlayRevs      = 2000
)

type overlayCache struct {
	mu          sync.Mutex
	revHead     string
	paths       []string
	revs        []gitwrite.Rev
	revsReady   bool
	eventsHead  string
	events      map[string][]Event
	global      []Event
	globalLimit int
	persist     *valkey.Client
	hydrated    bool
}

type overlaySnap struct {
	Head  string      `json:"head"`
	Paths []string    `json:"paths"`
	Revs  []storedRev `json:"revs"`
}

type storedRev struct {
	Hash    string            `json:"hash"`
	Message string            `json:"message"`
	Author  string            `json:"author"`
	When    time.Time         `json:"when"`
	Files   map[string]string `json:"files,omitempty"`
	Prev    map[string]string `json:"prev,omitempty"`
}

func newOverlayCache(addr string) *overlayCache {
	c := &overlayCache{events: map[string][]Event{}}
	if a := strings.TrimSpace(addr); a != "" {
		c.persist = &valkey.Client{Addr: a}
	}
	return c
}

// WatchOverlays keeps the overlay commit log warm: hydrate from Valkey, walk
// git off the request path, persist. History pages read from this snapshot.
func (s *Service) WatchOverlays(ctx context.Context) {
	if s == nil {
		return
	}
	s.initCaches()
	if s.overlays != nil {
		s.overlays.hydrate(ctx)
	}
	s.RefreshOverlays(ctx)
	every := s.OverlayEvery
	if every <= 0 {
		every = defaultOverlayEvery
	}
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.RefreshOverlays(ctx)
		}
	}
}

// RefreshOverlays fills the overlay commit log once. Tests use it instead of WatchOverlays.
func (s *Service) RefreshOverlays(ctx context.Context) {
	if s == nil || s.OpsRepo == "" || ctx.Err() != nil {
		return
	}
	if err := s.syncOverlayRevs(ctx); err != nil {
		slog.Warn("overlays refresh", "err", err)
	}
}

func (s *Service) syncOverlayRevs(ctx context.Context) error {
	s.initCaches()
	if s.overlays != nil {
		s.overlays.hydrate(ctx)
	}
	head, err := gitwrite.HeadHash(s.OpsRepo)
	if err != nil {
		return err
	}
	_, _, sorted := s.catalogOverlayPaths()
	c := s.overlays
	c.mu.Lock()
	ready := c.revsReady && c.revHead == head && slices.Equal(c.paths, sorted)
	oldHead := c.revHead
	oldRevs := c.revs
	samePaths := c.revsReady && slices.Equal(c.paths, sorted)
	c.mu.Unlock()
	if ready {
		return nil
	}

	var revs []gitwrite.Rev
	if samePaths && oldHead != "" && oldHead != head {
		newer, found, err := gitwrite.LogPathsAfter(ctx, s.OpsRepo, sorted, oldHead, 0)
		if err != nil {
			return err
		}
		if found {
			revs = append(slimRevs(newer), oldRevs...)
			if len(revs) > maxOverlayRevs {
				revs = revs[:maxOverlayRevs]
			}
			c.installRevs(head, sorted, revs)
			c.persistSnap()
			return nil
		}
	}

	revs, err = gitwrite.LogPaths(ctx, s.OpsRepo, sorted, 0)
	if err != nil {
		return err
	}
	revs = slimRevs(revs)
	if len(revs) > maxOverlayRevs {
		revs = revs[:maxOverlayRevs]
	}
	c.installRevs(head, sorted, revs)
	c.persistSnap()
	return nil
}

func (s *Service) catalogOverlayPaths() (paths []string, byPath map[string]overlayHit, sorted []string) {
	if s == nil || s.Catalog == nil {
		return nil, map[string]overlayHit{}, nil
	}
	paths, byPath = overlayIndex(s.Catalog.List())
	sorted = slices.Clone(paths)
	slices.Sort(sorted)
	return paths, byPath, sorted
}

func (c *overlayCache) installRevs(head string, paths []string, revs []gitwrite.Rev) {
	if c == nil {
		return
	}
	if revs == nil {
		revs = []gitwrite.Rev{}
	}
	c.mu.Lock()
	c.revHead = head
	c.paths = slices.Clone(paths)
	c.revs = revs
	c.revsReady = true
	c.eventsHead = ""
	c.events = map[string][]Event{}
	c.global = nil
	c.globalLimit = 0
	c.mu.Unlock()
}

func (c *overlayCache) hydrate(ctx context.Context) {
	if c == nil {
		return
	}
	c.mu.Lock()
	if c.hydrated || c.persist == nil {
		c.hydrated = true
		c.mu.Unlock()
		return
	}
	persist := c.persist
	c.mu.Unlock()

	raw, err := persist.Get(ctx, overlayKey)
	if err != nil {
		slog.Warn("overlay hydrate", "err", err)
		c.mu.Lock()
		c.hydrated = true
		c.mu.Unlock()
		return
	}
	var snap overlaySnap
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &snap); err != nil {
			slog.Warn("overlay hydrate decode", "err", err)
		}
	}
	c.mu.Lock()
	c.hydrated = true
	if !c.revsReady && snap.Head != "" {
		c.revHead = snap.Head
		c.paths = slices.Clone(snap.Paths)
		c.revs = loadRevs(snap.Revs)
		c.revsReady = true
	}
	c.mu.Unlock()
}

func (c *overlayCache) persistSnap() {
	if c == nil || c.persist == nil {
		return
	}
	c.mu.Lock()
	if !c.revsReady {
		c.mu.Unlock()
		return
	}
	snap := overlaySnap{Head: c.revHead, Paths: slices.Clone(c.paths), Revs: storeRevs(c.revs)}
	persist := c.persist
	c.mu.Unlock()
	raw, err := json.Marshal(snap)
	if err != nil {
		slog.Warn("overlay persist marshal", "err", err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := persist.Set(ctx, overlayKey, raw); err != nil {
		slog.Warn("overlay persist", "err", err)
	}
}

func (c *overlayCache) coversLocked(paths []string) bool {
	if c == nil || !c.revsReady {
		return false
	}
	have := make(map[string]struct{}, len(c.paths))
	for _, p := range c.paths {
		have[p] = struct{}{}
	}
	for _, p := range paths {
		if p == "" {
			continue
		}
		if _, ok := have[p]; !ok {
			return false
		}
	}
	return true
}

func (s *Service) cachedOverlayEvents(head, name string, limit int) ([]Event, bool) {
	if s == nil || s.overlays == nil || name == "" {
		return nil, false
	}
	c := s.overlays
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.eventsHead != head {
		return nil, false
	}
	ev, ok := c.events[name]
	if !ok {
		return nil, false
	}
	return clipEvents(ev, limit), true
}

func (s *Service) cachedGlobalEvents(head string, limit int) ([]Event, bool) {
	if s == nil || s.overlays == nil {
		return nil, false
	}
	c := s.overlays
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.eventsHead != head || c.global == nil {
		return nil, false
	}
	if len(c.global) >= limit || c.globalLimit >= limit {
		return clipEvents(c.global, limit), true
	}
	return nil, false
}

func (s *Service) deriveOverlayEvents(head string, d *spec.Deployable) ([]Event, bool) {
	if s == nil || s.overlays == nil || d == nil {
		return nil, false
	}
	paths, byPath := overlayIndex([]*spec.Deployable{d})
	c := s.overlays
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.revsReady || c.revHead != head || !c.coversLocked(paths) {
		return nil, false
	}
	ev := eventsFromRevs(c.revs, paths, byPath, maxHistoryLimit)
	if ev == nil {
		ev = []Event{}
	}
	if c.eventsHead != head {
		c.eventsHead = head
		c.events = map[string][]Event{}
		c.global = nil
		c.globalLimit = 0
	}
	c.events[d.Metadata.Name] = ev
	return ev, true
}

func (s *Service) deriveGlobalEvents(head string) ([]Event, bool) {
	if s == nil || s.overlays == nil || s.Catalog == nil {
		return nil, false
	}
	paths, byPath, sorted := s.catalogOverlayPaths()
	c := s.overlays
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.revsReady || c.revHead != head || !slices.Equal(c.paths, sorted) {
		return nil, false
	}
	ev := eventsFromRevs(c.revs, paths, byPath, maxHistoryLimit)
	if ev == nil {
		ev = []Event{}
	}
	if c.eventsHead != head {
		c.eventsHead = head
		c.events = map[string][]Event{}
	}
	c.global = ev
	c.globalLimit = maxHistoryLimit
	return ev, true
}

func (s *Service) storeOverlayEvents(head, name string, ev []Event) {
	if s == nil || s.overlays == nil || name == "" {
		return
	}
	if ev == nil {
		ev = []Event{}
	}
	c := s.overlays
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.eventsHead != head {
		c.eventsHead = head
		c.events = map[string][]Event{}
		c.global = nil
		c.globalLimit = 0
	}
	c.events[name] = ev
}

func (s *Service) storeGlobalEvents(head string, ev []Event, limit int) {
	if s == nil || s.overlays == nil {
		return
	}
	if ev == nil {
		ev = []Event{}
	}
	c := s.overlays
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.eventsHead != head {
		c.eventsHead = head
		c.events = map[string][]Event{}
	}
	c.global = ev
	c.globalLimit = limit
}

func slimRevs(revs []gitwrite.Rev) []gitwrite.Rev {
	if len(revs) == 0 {
		return revs
	}
	out := make([]gitwrite.Rev, len(revs))
	for i, rev := range revs {
		out[i] = slimRev(rev)
	}
	return out
}

func slimRev(rev gitwrite.Rev) gitwrite.Rev {
	files := map[string][]byte{}
	prev := map[string][]byte{}
	for p, b := range rev.Files {
		if !bytes.Equal(b, rev.Prev[p]) {
			files[p] = b
			if v, ok := rev.Prev[p]; ok {
				prev[p] = v
			}
		}
	}
	for p, b := range rev.Prev {
		if _, ok := rev.Files[p]; !ok {
			prev[p] = b
		}
	}
	rev.Files = files
	rev.Prev = prev
	return rev
}

func storeRevs(revs []gitwrite.Rev) []storedRev {
	out := make([]storedRev, 0, len(revs))
	for _, rev := range revs {
		out = append(out, storedRev{
			Hash:    rev.Hash,
			Message: rev.Message,
			Author:  rev.Author,
			When:    rev.When.UTC(),
			Files:   bytesToStrings(rev.Files),
			Prev:    bytesToStrings(rev.Prev),
		})
	}
	return out
}

func loadRevs(revs []storedRev) []gitwrite.Rev {
	out := make([]gitwrite.Rev, 0, len(revs))
	for _, rev := range revs {
		out = append(out, gitwrite.Rev{
			Hash:    rev.Hash,
			Message: rev.Message,
			Author:  rev.Author,
			When:    rev.When.UTC(),
			Files:   stringsToBytes(rev.Files),
			Prev:    stringsToBytes(rev.Prev),
		})
	}
	return out
}

func bytesToStrings(in map[string][]byte) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = string(v)
	}
	return out
}

func stringsToBytes(in map[string]string) map[string][]byte {
	if len(in) == 0 {
		return map[string][]byte{}
	}
	out := make(map[string][]byte, len(in))
	for k, v := range in {
		out[k] = []byte(v)
	}
	return out
}
