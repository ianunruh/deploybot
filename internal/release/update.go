package release

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/ianunruh/deploybot/internal/image"
	"github.com/ianunruh/deploybot/internal/render"
	"github.com/ianunruh/deploybot/internal/spec"
)

const (
	defaultListingTTL  = time.Hour
	defaultUpdateEvery = 15 * time.Minute
)

type UpdatePin struct {
	Tag     string `json:"tag,omitempty"`
	Digest  string `json:"digest,omitempty"`
	Compact string `json:"compact,omitempty"`
	Ref     string `json:"ref,omitempty"`
}

type UpdateNewest struct {
	Tag       string     `json:"tag,omitempty"`
	Digest    string     `json:"digest,omitempty"`
	Ref       string     `json:"ref,omitempty"`
	CreatedAt *time.Time `json:"createdAt,omitempty"`
}

type UpdateStatus struct {
	Name       string        `json:"name"`
	Namespace  string        `json:"namespace"`
	Project    string        `json:"project"`
	Repository string        `json:"repository"`
	Stage      string        `json:"stage"`
	Current    UpdatePin     `json:"current"`
	Newest     *UpdateNewest `json:"newest,omitempty"`
	Stale      bool          `json:"stale"`
	Auto       string        `json:"auto,omitempty"`
	CheckedAt  *time.Time    `json:"checkedAt,omitempty"`
	Error      string        `json:"error,omitempty"`
}

type UpdateSummary struct {
	Stale bool   `json:"stale"`
	Auto  string `json:"auto,omitempty"`
}

type UpdateList struct {
	Updates []UpdateStatus `json:"updates"`
	Apply   bool           `json:"apply"`
	Push    bool           `json:"push"`
	Sync    bool           `json:"sync"`
}

type updateState struct {
	mu       sync.Mutex
	ttl      time.Duration
	listings map[string]cachedListing
	lastAuto map[string]time.Time
}

type cachedListing struct {
	listing   image.Listing
	err       error
	checkedAt time.Time
}

type fetchMode int

const (
	fetchNone fetchMode = iota
	fetchExpired
	fetchAlways
)

func (s *Service) updates() *updateState {
	s.initCaches()
	return s.update
}

func formatAuto(d time.Duration) string {
	if d <= 0 {
		return ""
	}
	if d%time.Hour == 0 {
		return fmt.Sprintf("%dh", d/time.Hour)
	}
	return d.String()
}

func pinJSON(img image.Ref) UpdatePin {
	return UpdatePin{
		Tag:     img.Tag,
		Digest:  img.Digest,
		Compact: img.Compact(),
		Ref:     img.String(),
	}
}

func (s *Service) ListUpdates(ctx context.Context) UpdateList {
	out := UpdateList{Updates: []UpdateStatus{}, Apply: s != nil && s.Apply, Push: s != nil && s.Push, Sync: s != nil && s.Sync}
	if s == nil || s.Catalog == nil {
		return out
	}
	for _, d := range s.Catalog.List() {
		if !d.TracksRegistry() {
			continue
		}
		out.Updates = append(out.Updates, s.snapshotUpdate(ctx, d, fetchNone))
	}
	slices.SortStableFunc(out.Updates, func(a, b UpdateStatus) int {
		if a.Stale != b.Stale {
			if a.Stale {
				return -1
			}
			return 1
		}
		return strings.Compare(a.Name, b.Name)
	})
	return out
}

func (s *Service) UpdateSummary(d *spec.Deployable) *UpdateSummary {
	if d == nil || !d.TracksRegistry() {
		return nil
	}
	out := &UpdateSummary{Auto: formatAuto(d.AutoUpdate())}
	if s == nil {
		return out
	}
	st := s.snapshotUpdate(context.Background(), d, fetchNone)
	out.Stale = st.Stale
	return out
}

func (s *Service) CachedUpdate(ctx context.Context, d *spec.Deployable) *UpdateStatus {
	if d == nil || !d.TracksRegistry() {
		return nil
	}
	st := s.snapshotUpdate(ctx, d, fetchNone)
	return &st
}

func (s *Service) CheckUpdate(ctx context.Context, name string) (UpdateStatus, error) {
	d, err := s.Catalog.Get(name)
	if err != nil {
		return UpdateStatus{}, err
	}
	if !d.TracksRegistry() {
		return UpdateStatus{}, fmt.Errorf("%s does not opt into registry tracking (spec.update)", name)
	}
	return s.snapshotUpdate(ctx, d, fetchAlways), nil
}

func (s *Service) PinNewest(ctx context.Context, name string) (UpdateStatus, Mutation, error) {
	st, err := s.CheckUpdate(ctx, name)
	if err != nil {
		return UpdateStatus{}, Mutation{}, err
	}
	if st.Error != "" {
		return st, Mutation{}, errors.New(st.Error)
	}
	if st.Newest == nil || st.Newest.Ref == "" {
		return st, Mutation{}, fmt.Errorf("no published image for %s", name)
	}
	if !st.Stale {
		return st, Mutation{DryRun: !s.Apply}, nil
	}
	mut, err := s.Pin(ctx, name, st.Stage, st.Newest.Ref)
	if err != nil {
		return st, Mutation{}, err
	}
	return st, mut, nil
}

func (s *Service) snapshotUpdate(ctx context.Context, d *spec.Deployable, mode fetchMode) UpdateStatus {
	tree, err := s.workingTree(ctx, d)
	if err != nil {
		tree = render.Tree{}
	}
	st := s.updateFromTree(d, tree)
	s.applyListing(&st, d, ctx, mode)
	return st
}

func (s *Service) updateFromTree(d *spec.Deployable, tree render.Tree) UpdateStatus {
	st := UpdateStatus{
		Name:       d.Metadata.Name,
		Namespace:  d.Spec.Namespace,
		Project:    d.Spec.Argo.Project,
		Repository: d.Spec.Image.Repository,
		Stage:      d.BaseStage().Name,
		Auto:       formatAuto(d.AutoUpdate()),
	}
	if img, err := render.CurrentImage(tree, d, st.Stage); err == nil {
		st.Current = pinJSON(img)
	}
	return st
}

func (s *Service) applyListing(st *UpdateStatus, d *spec.Deployable, ctx context.Context, mode fetchMode) {
	item, ok := s.lookupListing(d.Spec.Image.Repository)
	switch mode {
	case fetchAlways:
		item = s.fetchListing(ctx, d)
	case fetchExpired:
		if !ok || !s.listingFresh(item) {
			item = s.fetchListing(ctx, d)
		}
	default:
		if !ok {
			return
		}
	}
	if !item.checkedAt.IsZero() {
		t := item.checkedAt.UTC()
		st.CheckedAt = &t
	}
	if item.err != nil {
		st.Error = item.err.Error()
		return
	}
	versions := item.listing.Versions
	if re := d.UpdateMatch(); re != nil {
		matched := image.Matching(versions, re)
		if len(versions) > 0 && len(matched) == 0 {
			st.Error = fmt.Sprintf("no published tags matching %q", strings.TrimSpace(d.Spec.Update.Match))
			return
		}
		versions = matched
	}
	newest, ok := image.Newest(versions)
	if !ok {
		return
	}
	n := &UpdateNewest{Tag: newest.Tag, Digest: newest.Digest, Ref: newest.Ref}
	if !newest.CreatedAt.IsZero() {
		t := newest.CreatedAt.UTC()
		n.CreatedAt = &t
	}
	st.Newest = n
	if st.Current.Ref == "" {
		return
	}
	current, err := image.Parse(st.Current.Ref)
	if err != nil {
		return
	}
	candidate := image.Ref{Repository: newest.Repository, Tag: newest.Tag, Digest: newest.Digest}
	st.Stale = !current.SameRelease(candidate)
}

func (s *Service) lookupListing(repository string) (cachedListing, bool) {
	st := s.updates()
	st.mu.Lock()
	defer st.mu.Unlock()
	got, ok := st.listings[image.CanonicalRepository(repository)]
	return got, ok
}

func (s *Service) listingFresh(item cachedListing) bool {
	if item.checkedAt.IsZero() {
		return false
	}
	return time.Since(item.checkedAt) < s.updates().ttl
}

func (s *Service) fetchListing(ctx context.Context, d *spec.Deployable) cachedListing {
	item := cachedListing{checkedAt: time.Now().UTC()}
	if s.Images == nil {
		item.err = fmt.Errorf("image listing is not configured")
	} else {
		listing, err := s.Images.List(ctx, d.Spec.Image.Repository, d.Spec.Image.Tag)
		item.listing = listing
		item.err = err
	}
	st := s.updates()
	st.mu.Lock()
	st.listings[image.CanonicalRepository(d.Spec.Image.Repository)] = item
	st.mu.Unlock()
	return item
}

func (s *Service) RefreshUpdates(ctx context.Context) {
	if s == nil || s.Catalog == nil {
		return
	}
	for _, d := range s.Catalog.List() {
		if ctx.Err() != nil {
			return
		}
		if d.TracksRegistry() {
			s.fetchListing(ctx, d)
		}
	}
}

func (s *Service) WatchUpdates(ctx context.Context) {
	if s == nil {
		return
	}
	every := s.UpdateEvery
	if every <= 0 {
		every = defaultUpdateEvery
	}
	s.ReconcileUpdates(ctx)
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.ReconcileUpdates(ctx)
		}
	}
}

func (s *Service) ReconcileUpdates(ctx context.Context) {
	if s == nil || s.Catalog == nil {
		return
	}
	if ctx.Err() != nil {
		return
	}
	now := time.Now()
	for _, d := range s.Catalog.List() {
		if ctx.Err() != nil {
			return
		}
		if !d.TracksRegistry() {
			continue
		}
		st := s.snapshotUpdate(ctx, d, fetchExpired)
		auto := d.AutoUpdate()
		if auto <= 0 || !s.Apply || !s.AutoPin {
			continue
		}
		if !s.autoDue(d.Metadata.Name, auto, now) {
			continue
		}
		s.markAuto(d.Metadata.Name, now)
		if st.Error != "" || !st.Stale || st.Newest == nil || st.Newest.Ref == "" {
			continue
		}
		slog.Info("auto-pin", "deployable", d.Metadata.Name, "stage", st.Stage, "image", st.Newest.Ref)
		if _, err := s.WithActor(ActorAutoPin()).Pin(ctx, d.Metadata.Name, st.Stage, st.Newest.Ref); err != nil {
			slog.Warn("auto-pin", "deployable", d.Metadata.Name, "err", err)
		}
	}
}

func (s *Service) autoDue(name string, every time.Duration, now time.Time) bool {
	st := s.updates()
	st.mu.Lock()
	defer st.mu.Unlock()
	last, ok := st.lastAuto[name]
	if !ok || last.IsZero() {
		return true
	}
	return !last.Add(every).After(now)
}

func (s *Service) markAuto(name string, now time.Time) {
	st := s.updates()
	st.mu.Lock()
	st.lastAuto[name] = now
	st.mu.Unlock()
}
