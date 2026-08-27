package release

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ianunruh/deploybot/internal/argo"
	"github.com/ianunruh/deploybot/internal/image"
	"github.com/ianunruh/deploybot/internal/kube"
	"github.com/ianunruh/deploybot/internal/render"
	"github.com/ianunruh/deploybot/internal/spec"
)

const statusArgoTimeout = 4 * time.Second

type StageStatus struct {
	Name          string         `json:"name"`
	Hostname      string         `json:"hostname"`
	Image         string         `json:"image,omitempty"`
	Sync          string         `json:"sync"`
	Health        string         `json:"health"`
	Revision      string         `json:"revision,omitempty"`
	Message       string         `json:"message,omitempty"`
	DeployedAt    *time.Time     `json:"deployedAt,omitempty"`
	PinnedAt      *time.Time     `json:"pinnedAt,omitempty"`
	PreviousImage string         `json:"previousImage,omitempty"`
	PreviousRef   string         `json:"previousRef,omitempty"`
	ArgoURL       string         `json:"argoURL,omitempty"`
	HeadlampURL   string         `json:"headlampURL,omitempty"`
	GrafanaURL    string         `json:"grafanaURL,omitempty"`
	LogsURL       string         `json:"logsURL,omitempty"`
	Workload      *kube.Workload `json:"workload,omitempty"`
	// Connected is set when a live snapshot exists. False means the filler
	// could not reach the cluster; Health/Sync are last-known.
	Connected *bool      `json:"connected,omitempty"`
	UpdatedAt *time.Time `json:"updatedAt,omitempty"`
}

type Status struct {
	Name       string        `json:"name"`
	Namespace  string        `json:"namespace"`
	Project    string        `json:"project"`
	Summary    string        `json:"summary,omitempty"`
	Icon       string        `json:"icon,omitempty"`
	DocsURL    string        `json:"docsURL,omitempty"`
	ImageRepo  string        `json:"imageRepo"`
	RepoURL    string        `json:"repoURL,omitempty"`
	ProjectURL string        `json:"projectURL,omitempty"`
	Source     bool          `json:"source,omitempty"`
	Stages     []StageStatus `json:"stages"`
	Flow       Flow          `json:"flow"`
	Apply      bool          `json:"apply"`
	Push       bool          `json:"push"`
	Sync       bool          `json:"sync"`
	Update     *UpdateStatus `json:"update,omitempty"`
	Pause      *PauseFile    `json:"pause,omitempty"`
}

// Live is a catalog-list snapshot: newest Argo deployedAt, stage health, and
// the current release flow. Source commit lookup is omitted.
type Live struct {
	DeployedAt *time.Time    `json:"deployedAt,omitempty"`
	Stages     []StageStatus `json:"stages"`
	Flow       Flow          `json:"flow"`
}

func (s *Service) Status(ctx context.Context, name string) (Status, error) {
	return s.status(ctx, name, false)
}

// LiveStatus is Status plus source commit lookup. Argo live state comes
// from the WatchLive snapshot; missing snapshots stay "waiting".
func (s *Service) LiveStatus(ctx context.Context, name string) (Status, error) {
	return s.status(ctx, name, true)
}

func (s *Service) status(ctx context.Context, name string, withSource bool) (Status, error) {
	d, err := s.Catalog.Get(name)
	if err != nil {
		return Status{}, err
	}
	tree, err := s.workingTree(ctx, d)
	if err != nil {
		return Status{}, err
	}
	out := s.buildStatus(ctx, d, tree)
	// Live kube and GitHub workflows are separate endpoints so catalog
	// list, WatchFlows, and the core status poll do not wait on them.
	if withSource && out.Flow.Tag != "" && d.HasSourceCommits() {
		sctx, cancel := context.WithTimeout(ctx, sourceCommitTimeout)
		out.Flow.Source = s.resolveSource(sctx, d.Spec.Links.RepoURL, out.Flow.Tag)
		cancel()
	}
	return out, nil
}

// Latest returns live flow, stages, and newest Argo deployedAt for each
// catalog deployable. Missing Argo clients, git, and Get errors are skipped.
func (s *Service) Latest(ctx context.Context) map[string]Live {
	out := map[string]Live{}
	if s == nil || s.Catalog == nil {
		return out
	}
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, d := range s.Catalog.List() {
		wg.Go(func() {
			tree, err := s.workingTree(ctx, d)
			if err != nil {
				tree = render.Tree{}
			}
			st := s.buildStatus(ctx, d, tree)
			live := Live{
				DeployedAt: newestDeployedAt(st.Stages),
				Stages:     st.Stages,
				Flow:       st.Flow,
			}
			mu.Lock()
			out[d.Metadata.Name] = live
			mu.Unlock()
		})
	}
	wg.Wait()
	return out
}

func (s *Service) buildStatus(ctx context.Context, d *spec.Deployable, tree render.Tree) Status {
	out := Status{
		Name:       d.Metadata.Name,
		Namespace:  d.Spec.Namespace,
		Project:    d.Spec.Argo.Project,
		Summary:    d.Spec.Summary,
		Icon:       d.Spec.Links.Icon,
		DocsURL:    d.Spec.Links.DocsURL,
		ImageRepo:  d.Spec.Image.Repository,
		RepoURL:    d.Spec.Links.RepoURL,
		ProjectURL: d.Spec.Links.ProjectURL,
		Source:     d.HasSourceCommits(),
		Apply:      s.Apply,
		Push:       s.Push,
		Sync:       s.Sync,
	}
	out.Stages = make([]StageStatus, len(d.Spec.Stages))
	refs := make([]image.Ref, len(d.Spec.Stages))
	events, err := s.overlayChanges(ctx, d, defaultHistoryLimit)
	if err != nil {
		events = nil
	}
	for i, st := range d.Spec.Stages {
		ss := StageStatus{
			Name:     st.Name,
			Hostname: st.Hostname,
			Sync:     "unknown",
			Health:   "unknown",
		}
		ss.HeadlampURL, ss.GrafanaURL, ss.LogsURL = s.Observability(st.Name, d.Spec.Namespace)
		if img, err := render.CurrentImage(tree, d, st.Name); err == nil {
			ss.Image = img.Compact()
			refs[i] = img
			ss.PinnedAt = pinTime(events, st.Name, img)
			ss.PreviousImage, ss.PreviousRef = previousPin(events, st.Name, img, d.Spec.Image.Repository)
		}
		if s.Argo != nil {
			if c := s.Argo.ForStage(st.Name); c != nil {
				ss.ArgoURL = argo.AppURL(c, d.Spec.Argo.Name)
				s.fillStageArgo(st.Name, d.Spec.Argo.Name, &ss)
			}
		}
		out.Stages[i] = ss
	}
	snaps := make([]stageSnap, len(d.Spec.Stages))
	for i, st := range d.Spec.Stages {
		disconnected := out.Stages[i].Connected != nil && !*out.Stages[i].Connected
		snaps[i] = stageSnap{
			name:         st.Name,
			ref:          refs[i],
			health:       out.Stages[i].Health,
			pinnedAt:     out.Stages[i].PinnedAt,
			policy:       st.Promote,
			hasArgo:      s.Argo != nil && s.Argo.ForStage(st.Name) != nil,
			disconnected: disconnected,
		}
	}
	if pause := s.CurrentPause(); !pause.Empty() {
		out.Pause = &pause
		for i, st := range d.Spec.Stages {
			snaps[i].paused = pause.Hit(d.Metadata.Name, st.Name) != nil
		}
	}
	out.Flow = buildFlow(snaps, time.Now().UTC())
	if d.TracksRegistry() {
		st := s.updateFromTree(d, tree)
		s.applyListing(&st, d, ctx, fetchNone)
		out.Update = &st
	}
	return out
}

func (s *Service) fillStageArgo(stage, app string, ss *StageStatus) {
	snap, ok := s.liveSnapshot(stage)
	if !ok {
		connected := false
		ss.Connected = &connected
		ss.Message = "waiting for live snapshot"
		return
	}
	connected := snap.Connected
	ss.Connected = &connected
	if !snap.UpdatedAt.IsZero() {
		t := snap.UpdatedAt
		ss.UpdatedAt = &t
	}
	if got, ok := snap.Apps[app]; ok {
		ss.Health = got.Health
		ss.Sync = got.Sync
		ss.Revision = got.Revision
		ss.Message = got.Message
		ss.DeployedAt = got.DeployedAt
		if !snap.Connected && snap.Message != "" && ss.Message == "" {
			ss.Message = snap.Message
		}
		return
	}
	if snap.Message != "" {
		ss.Message = snap.Message
		return
	}
	ss.Message = fmt.Sprintf("app %s not found", app)
}

func newestDeployedAt(stages []StageStatus) *time.Time {
	var latest *time.Time
	for _, st := range stages {
		if st.DeployedAt == nil {
			continue
		}
		if latest == nil || st.DeployedAt.After(*latest) {
			latest = st.DeployedAt
		}
	}
	return latest
}
