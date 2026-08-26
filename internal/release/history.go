package release

import (
	"bytes"
	"context"
	"strings"
	"time"

	"github.com/ianunruh/deploybot/internal/gitwrite"
	"github.com/ianunruh/deploybot/internal/image"
	"github.com/ianunruh/deploybot/internal/render"
	"github.com/ianunruh/deploybot/internal/spec"
)

const (
	EventPin       = "pin"
	EventPromote   = "promote"
	EventRollback  = "rollback"
	EventOverlay   = "overlay"
	EventReconcile = "reconcile"

	maxPathRevs         = 80
	maxGlobalPathRevs   = 200
	defaultHistoryLimit = 50
	maxHistoryLimit     = 200
)

type Event struct {
	At         time.Time `json:"at"`
	Kind       string    `json:"kind"`
	Deployable string    `json:"deployable,omitempty"`
	Namespace  string    `json:"namespace,omitempty"`
	Project    string    `json:"project,omitempty"`
	Stage      string    `json:"stage"`
	Image      string    `json:"image"`
	Digest     string    `json:"digest,omitempty"`
	Tag        string    `json:"tag,omitempty"`
	Commit     string    `json:"commit"`
	CommitURL  string    `json:"commitURL,omitempty"`
	Author     string    `json:"author,omitempty"`
}

type GlobalHistory struct {
	Events []Event `json:"events"`
}

type ReleaseStage struct {
	At        time.Time `json:"at"`
	Kind      string    `json:"kind"`
	Commit    string    `json:"commit,omitempty"`
	CommitURL string    `json:"commitURL,omitempty"`
}

type SourceCommit struct {
	SHA     string `json:"sha,omitempty"`
	Message string `json:"message,omitempty"`
	Author  string `json:"author,omitempty"`
	URL     string `json:"url,omitempty"`
}

type Release struct {
	Image   string                  `json:"image"`
	Digest  string                  `json:"digest,omitempty"`
	Tag     string                  `json:"tag,omitempty"`
	Current bool                    `json:"current,omitempty"`
	Source  SourceCommit            `json:"source,omitempty"`
	Stages  map[string]ReleaseStage `json:"stages"`
}

type History struct {
	Events   []Event   `json:"events"`
	Releases []Release `json:"releases"`
}

func (s *Service) History(ctx context.Context, name string, limit int) (History, error) {
	d, err := s.Catalog.Get(name)
	if err != nil {
		return History{}, err
	}
	limit = clampHistoryLimit(limit)
	events, err := s.overlayChanges(ctx, d, limit)
	if err != nil {
		return History{}, err
	}
	if events == nil {
		events = []Event{}
	}
	var current image.Ref
	if tree, err := s.workingTree(ctx, d); err == nil {
		if img, err := render.CurrentImage(tree, d, d.BaseStage().Name); err == nil {
			current = img
		}
	}
	releases := groupReleases(events, current)
	if d.HasSourceCommits() {
		s.attachSources(ctx, d.Spec.Links.RepoURL, releases)
	}
	return History{Events: events, Releases: releases}, nil
}

func clampHistoryLimit(limit int) int {
	if limit <= 0 {
		return defaultHistoryLimit
	}
	if limit > maxHistoryLimit {
		return maxHistoryLimit
	}
	return limit
}

func (s *Service) ListHistory(ctx context.Context, limit int) (GlobalHistory, error) {
	limit = clampHistoryLimit(limit)
	events, err := s.allOverlayChanges(ctx, limit)
	if err != nil {
		return GlobalHistory{}, err
	}
	if events == nil {
		events = []Event{}
	}
	return GlobalHistory{Events: events}, nil
}

func (s *Service) overlayChanges(ctx context.Context, d *spec.Deployable, limit int) ([]Event, error) {
	if s == nil || s.OpsRepo == "" || d == nil {
		return nil, nil
	}
	limit = clampHistoryLimit(limit)
	head, err := gitwrite.HeadHash(s.OpsRepo)
	if err != nil {
		return s.computeOverlayChanges(ctx, d, limit)
	}
	s.initCaches()
	c := s.overlays
	c.mu.Lock()
	defer c.mu.Unlock()
	resetOverlayCache(c, head)
	if ev, ok := c.events[d.Metadata.Name]; ok {
		return clipEvents(ev, limit), nil
	}
	ev, err := s.computeOverlayChanges(ctx, d, defaultHistoryLimit)
	if err != nil {
		return nil, err
	}
	c.events[d.Metadata.Name] = ev
	return clipEvents(ev, limit), nil
}

func (s *Service) allOverlayChanges(ctx context.Context, limit int) ([]Event, error) {
	if s == nil || s.OpsRepo == "" || s.Catalog == nil {
		return nil, nil
	}
	limit = clampHistoryLimit(limit)
	head, err := gitwrite.HeadHash(s.OpsRepo)
	if err != nil {
		return s.computeAllOverlayChanges(ctx, limit)
	}
	s.initCaches()
	c := s.overlays
	c.mu.Lock()
	defer c.mu.Unlock()
	resetOverlayCache(c, head)
	if c.global != nil && (len(c.global) >= limit || c.globalLimit >= limit) {
		return clipEvents(c.global, limit), nil
	}
	ev, err := s.computeAllOverlayChanges(ctx, limit)
	if err != nil {
		return nil, err
	}
	if ev == nil {
		ev = []Event{}
	}
	c.global = ev
	c.globalLimit = limit
	return clipEvents(ev, limit), nil
}

func resetOverlayCache(c *overlayCache, head string) {
	if c == nil || c.head == head {
		return
	}
	c.head = head
	c.events = map[string][]Event{}
	c.global = nil
	c.globalLimit = 0
}

func clipEvents(events []Event, limit int) []Event {
	if limit <= 0 || len(events) <= limit {
		return events
	}
	return events[:limit]
}

func (s *Service) computeOverlayChanges(ctx context.Context, d *spec.Deployable, limit int) ([]Event, error) {
	paths, byPath := overlayIndex([]*spec.Deployable{d})
	return s.overlayEvents(ctx, paths, byPath, maxPathRevs, limit)
}

func (s *Service) computeAllOverlayChanges(ctx context.Context, limit int) ([]Event, error) {
	if s.Catalog == nil {
		return nil, nil
	}
	paths, byPath := overlayIndex(s.Catalog.List())
	return s.overlayEvents(ctx, paths, byPath, gitRevsFor(limit), limit)
}

func gitRevsFor(eventLimit int) int {
	n := eventLimit * 2
	if n < maxPathRevs {
		n = maxPathRevs
	}
	if n > maxGlobalPathRevs {
		n = maxGlobalPathRevs
	}
	return n
}

type overlayHit struct {
	d     *spec.Deployable
	stage string
}

func overlayIndex(deployables []*spec.Deployable) ([]string, map[string]overlayHit) {
	byPath := map[string]overlayHit{}
	var paths []string
	for _, d := range deployables {
		if d == nil {
			continue
		}
		for _, st := range d.Spec.Stages {
			p := render.OverlayKustomizationPath(d, st.Name)
			if p == "" {
				continue
			}
			if _, ok := byPath[p]; ok {
				continue
			}
			byPath[p] = overlayHit{d: d, stage: st.Name}
			paths = append(paths, p)
		}
	}
	return paths, byPath
}

func (s *Service) overlayEvents(ctx context.Context, paths []string, byPath map[string]overlayHit, revsLimit, eventLimit int) ([]Event, error) {
	if s == nil || s.OpsRepo == "" || len(paths) == 0 {
		return nil, nil
	}
	revs, err := gitwrite.LogPaths(ctx, s.OpsRepo, paths, revsLimit)
	if err != nil {
		return nil, err
	}
	var events []Event
	for _, rev := range revs {
		kind := eventKind(rev.Message)
		if kind == EventReconcile {
			continue
		}
		for _, p := range paths {
			hit, ok := byPath[p]
			if !ok {
				continue
			}
			ev, ok := overlayChange(hit, p, kind, rev)
			if !ok {
				continue
			}
			events = append(events, ev)
			if eventLimit > 0 && len(events) >= eventLimit {
				return events, nil
			}
		}
	}
	return events, nil
}

func overlayChange(hit overlayHit, path, kind string, rev gitwrite.Rev) (Event, bool) {
	if hit.d == nil {
		return Event{}, false
	}
	cur, ok := overlayImage(hit.d, hit.stage, rev.Files[path])
	if !ok {
		return Event{}, false
	}
	if prev, prevOK := overlayImage(hit.d, hit.stage, rev.Prev[path]); prevOK && cur.ReleaseKey() == prev.ReleaseKey() {
		return Event{}, false
	}
	return Event{
		At:         rev.When,
		Kind:       kind,
		Deployable: hit.d.Metadata.Name,
		Namespace:  hit.d.Spec.Namespace,
		Project:    hit.d.Spec.Argo.Project,
		Stage:      hit.stage,
		Image:      cur.Compact(),
		Digest:     cur.Digest,
		Tag:        cur.Tag,
		Commit:     rev.Hash,
		CommitURL:  gitCommitURL(hit.d.Spec.Git.RepoURL, rev.Hash),
		Author:     rev.Author,
	}, true
}

func overlayImage(d *spec.Deployable, stage string, blob []byte) (image.Ref, bool) {
	if len(bytes.TrimSpace(blob)) == 0 {
		return image.Ref{}, false
	}
	tree := render.Tree{render.OverlayKustomizationPath(d, stage): blob}
	ref, err := render.CurrentImage(tree, d, stage)
	if err != nil {
		return image.Ref{}, false
	}
	return ref, true
}

func eventKind(message string) string {
	msg := strings.TrimSpace(message)
	if i := strings.IndexByte(msg, '\n'); i >= 0 {
		msg = strings.TrimSpace(msg[:i])
	}
	switch {
	case strings.HasPrefix(msg, "pin "):
		return EventPin
	case strings.HasPrefix(msg, "promote "):
		return EventPromote
	case strings.HasPrefix(msg, "rollback "):
		return EventRollback
	case strings.HasPrefix(msg, "reconcile "):
		return EventReconcile
	default:
		return EventOverlay
	}
}

func gitCommitURL(repoURL, hash string) string {
	repoURL = strings.TrimSuffix(strings.TrimRight(repoURL, "/"), ".git")
	if repoURL == "" || hash == "" {
		return ""
	}
	return repoURL + "/commit/" + hash
}

func groupReleases(events []Event, current image.Ref) []Release {
	order := make([]string, 0)
	by := map[string]*Release{}
	keyOf := func(e Event) string {
		if e.Digest != "" {
			return e.Digest
		}
		return e.Image
	}
	for _, e := range events {
		k := keyOf(e)
		if _, ok := by[k]; ok {
			continue
		}
		r := &Release{
			Image:  e.Image,
			Digest: e.Digest,
			Tag:    e.Tag,
			Stages: map[string]ReleaseStage{},
		}
		if current.Digest != "" && e.Digest == current.Digest {
			r.Current = true
		} else if current.Digest == "" && current.ReleaseKey() != "" && k == current.ReleaseKey() {
			r.Current = true
		}
		by[k] = r
		order = append(order, k)
	}
	for i := len(events) - 1; i >= 0; i-- {
		e := events[i]
		r := by[keyOf(e)]
		if _, ok := r.Stages[e.Stage]; ok {
			continue
		}
		r.Stages[e.Stage] = ReleaseStage{
			At:        e.At,
			Kind:      e.Kind,
			Commit:    e.Commit,
			CommitURL: e.CommitURL,
		}
	}
	out := make([]Release, 0, len(order))
	for _, k := range order {
		out = append(out, *by[k])
	}
	return out
}

func previousPin(events []Event, stage string, current image.Ref, repo string) (compact, full string) {
	if current.IsZero() {
		return "", ""
	}
	seenCurrent := false
	for _, e := range events {
		if e.Stage != stage {
			continue
		}
		got := eventRef(repo, e)
		if !seenCurrent {
			if historicRelease(got, current) {
				seenCurrent = true
			}
			continue
		}
		if historicRelease(got, current) {
			continue
		}
		return e.Image, got.String()
	}
	return "", ""
}

func pinTime(events []Event, stage string, current image.Ref) *time.Time {
	if current.IsZero() {
		return nil
	}
	for _, e := range events {
		if e.Stage != stage {
			continue
		}
		got := image.Ref{Repository: current.Repository, Tag: e.Tag, Digest: e.Digest}
		if !got.SameRelease(current) {
			continue
		}
		t := e.At
		return &t
	}
	return nil
}
