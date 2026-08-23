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
	EventOverlay   = "overlay"
	EventReconcile = "reconcile"

	maxPathRevs         = 80
	defaultHistoryLimit = 50
	maxHistoryLimit     = 200
)

type Event struct {
	At        time.Time `json:"at"`
	Kind      string    `json:"kind"`
	Stage     string    `json:"stage"`
	Image     string    `json:"image"`
	Digest    string    `json:"digest,omitempty"`
	Tag       string    `json:"tag,omitempty"`
	Commit    string    `json:"commit"`
	CommitURL string    `json:"commitURL,omitempty"`
	Author    string    `json:"author,omitempty"`
}

type ReleaseStage struct {
	At        time.Time `json:"at"`
	Kind      string    `json:"kind"`
	Commit    string    `json:"commit,omitempty"`
	CommitURL string    `json:"commitURL,omitempty"`
}

type Release struct {
	Image   string                  `json:"image"`
	Digest  string                  `json:"digest,omitempty"`
	Tag     string                  `json:"tag,omitempty"`
	Current bool                    `json:"current,omitempty"`
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
	if limit <= 0 {
		limit = defaultHistoryLimit
	}
	if limit > maxHistoryLimit {
		limit = maxHistoryLimit
	}
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
	return History{Events: events, Releases: groupReleases(events, current)}, nil
}

func (s *Service) overlayChanges(ctx context.Context, d *spec.Deployable, limit int) ([]Event, error) {
	if s == nil || s.OpsRepo == "" || d == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = defaultHistoryLimit
	}
	head, err := gitwrite.HeadHash(s.OpsRepo)
	if err != nil {
		return s.computeOverlayChanges(ctx, d, limit)
	}
	if s.overlays == nil {
		s.overlays = &overlayCache{events: map[string][]Event{}}
	}
	c := s.overlays
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.head != head {
		c.head = head
		c.events = map[string][]Event{}
	}
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

func clipEvents(events []Event, limit int) []Event {
	if limit <= 0 || len(events) <= limit {
		return events
	}
	return events[:limit]
}

func (s *Service) computeOverlayChanges(ctx context.Context, d *spec.Deployable, limit int) ([]Event, error) {
	paths := make([]string, 0, len(d.Spec.Stages))
	stageOf := make(map[string]string, len(d.Spec.Stages))
	for _, st := range d.Spec.Stages {
		p := render.OverlayKustomizationPath(d, st.Name)
		paths = append(paths, p)
		stageOf[p] = st.Name
	}
	revs, err := gitwrite.LogPaths(ctx, s.OpsRepo, paths, maxPathRevs)
	if err != nil {
		return nil, err
	}
	var events []Event
	for _, rev := range revs {
		kind := eventKind(rev.Message)
		if kind == EventReconcile {
			continue
		}
		url := gitCommitURL(d.Spec.Git.RepoURL, rev.Hash)
		for _, p := range paths {
			stage := stageOf[p]
			cur, ok := overlayImage(d, stage, rev.Files[p])
			if !ok {
				continue
			}
			if prev, prevOK := overlayImage(d, stage, rev.Prev[p]); prevOK && cur.ReleaseKey() == prev.ReleaseKey() {
				continue
			}
			events = append(events, Event{
				At:        rev.When,
				Kind:      kind,
				Stage:     stage,
				Image:     cur.Compact(),
				Digest:    cur.Digest,
				Tag:       cur.Tag,
				Commit:    rev.Hash,
				CommitURL: url,
				Author:    rev.Author,
			})
			if limit > 0 && len(events) >= limit {
				return events, nil
			}
		}
	}
	return events, nil
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
