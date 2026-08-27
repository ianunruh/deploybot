package release

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ianunruh/deploybot/internal/gitwrite"
	"github.com/ianunruh/deploybot/internal/render"
	"github.com/ianunruh/deploybot/internal/yamlx"
)

// PausePath is the ops-repo file that records operator pauses. It is not a
// Kubernetes object and must not be referenced by an Argo Application.
const PausePath = "deploybot/pause.yaml"

const (
	pauseScopeAll      = "all"
	pauseScopeStage    = "stage"
	pauseScopeApp      = "app"
	pauseScopeAppStage = "app-stage"

	maxPauseReason = 160
)

// ErrPaused is the sentinel unwrap of PauseError. Automated pin and
// auto-promote return it when the target stage is frozen. Console and CLI
// writes still go through.
var ErrPaused = errors.New("deployments paused")

const pauseHeader = "# deploybot pause interlock. Not a Kubernetes object; Argo must not apply this file.\n"

// PauseEntry is one independent pause (who / when / why).
type PauseEntry struct {
	At     time.Time `json:"at,omitempty" yaml:"at,omitempty"`
	By     string    `json:"by,omitempty" yaml:"by,omitempty"`
	Reason string    `json:"reason,omitempty" yaml:"reason,omitempty"`
}

// AppPause is an app-wide freeze and/or per-stage freezes for that app.
// At set means every stage of the app is paused.
type AppPause struct {
	At     time.Time             `json:"at,omitempty" yaml:"at,omitempty"`
	By     string                `json:"by,omitempty" yaml:"by,omitempty"`
	Reason string                `json:"reason,omitempty" yaml:"reason,omitempty"`
	Stages map[string]PauseEntry `json:"stages,omitempty" yaml:"stages,omitempty"`
}

// PauseFile is deploybot/pause.yaml. Missing or empty means nothing is paused.
// Selectors stack and lift independently: all, stages.<stage>, apps.<name>,
// apps.<name>.stages.<stage>.
type PauseFile struct {
	All    *PauseEntry           `json:"all,omitempty" yaml:"all,omitempty"`
	Stages map[string]PauseEntry `json:"stages,omitempty" yaml:"stages,omitempty"`
	Apps   map[string]AppPause   `json:"apps,omitempty" yaml:"apps,omitempty"`
}

func (f PauseFile) Empty() bool {
	return f.All == nil && len(f.Stages) == 0 && len(f.Apps) == 0
}

// PauseError is returned when an automated pin or auto-promote would write a
// paused stage.
type PauseError struct {
	Scope  string    `json:"scope"`
	App    string    `json:"app,omitempty"`
	Stage  string    `json:"stage,omitempty"`
	Reason string    `json:"reason,omitempty"`
	By     string    `json:"by,omitempty"`
	At     time.Time `json:"at,omitempty"`
}

func (e *PauseError) Error() string {
	if e == nil {
		return ErrPaused.Error()
	}
	label := "deployments paused"
	switch e.Scope {
	case pauseScopeAppStage:
		label = e.App + "/" + e.Stage + " is paused"
	case pauseScopeApp:
		label = e.App + " is paused"
	case pauseScopeStage:
		label = e.Stage + " is paused"
	}
	if e.Reason != "" {
		return label + " (" + e.Reason + ")"
	}
	return label
}

func (e *PauseError) Unwrap() error { return ErrPaused }

func pauseErr(scope, app, stage string, e PauseEntry) *PauseError {
	return &PauseError{
		Scope:  scope,
		App:    app,
		Stage:  stage,
		Reason: e.Reason,
		By:     e.By,
		At:     e.At,
	}
}

// Hit is the most specific pause that blocks a write to (app, stage).
func (f PauseFile) Hit(app, stage string) *PauseError {
	if a, ok := f.Apps[app]; ok {
		if stage != "" {
			if e, ok := a.Stages[stage]; ok {
				return pauseErr(pauseScopeAppStage, app, stage, e)
			}
		}
		if !a.At.IsZero() {
			return pauseErr(pauseScopeApp, app, "", PauseEntry{At: a.At, By: a.By, Reason: a.Reason})
		}
	}
	if stage != "" {
		if e, ok := f.Stages[stage]; ok {
			return pauseErr(pauseScopeStage, "", stage, e)
		}
	}
	if f.All != nil {
		return pauseErr(pauseScopeAll, "", "", *f.All)
	}
	return nil
}

func (f PauseFile) has(name, stage string) bool {
	switch {
	case name == "" && stage == "":
		return f.All != nil
	case name == "":
		_, ok := f.Stages[stage]
		return ok
	case stage == "":
		a, ok := f.Apps[name]
		return ok && !a.At.IsZero()
	default:
		a, ok := f.Apps[name]
		if !ok {
			return false
		}
		_, ok = a.Stages[stage]
		return ok
	}
}

func (f *PauseFile) set(name, stage string, e PauseEntry) {
	switch {
	case name == "" && stage == "":
		cp := e
		f.All = &cp
	case name == "":
		if f.Stages == nil {
			f.Stages = map[string]PauseEntry{}
		}
		f.Stages[stage] = e
	case stage == "":
		if f.Apps == nil {
			f.Apps = map[string]AppPause{}
		}
		app := f.Apps[name]
		app.At, app.By, app.Reason = e.At, e.By, e.Reason
		f.Apps[name] = app
	default:
		if f.Apps == nil {
			f.Apps = map[string]AppPause{}
		}
		app := f.Apps[name]
		if app.Stages == nil {
			app.Stages = map[string]PauseEntry{}
		}
		app.Stages[stage] = e
		f.Apps[name] = app
	}
}

func (f *PauseFile) clear(name, stage string) {
	switch {
	case name == "" && stage == "":
		f.All = nil
	case name == "":
		delete(f.Stages, stage)
	case stage == "":
		app := f.Apps[name]
		app.At, app.By, app.Reason = time.Time{}, "", ""
		f.Apps[name] = app
	default:
		app := f.Apps[name]
		delete(app.Stages, stage)
		f.Apps[name] = app
	}
	f.compact()
}

func (f *PauseFile) compact() {
	if len(f.Stages) == 0 {
		f.Stages = nil
	}
	for name, app := range f.Apps {
		if len(app.Stages) == 0 {
			app.Stages = nil
		}
		if app.At.IsZero() && app.By == "" && app.Reason == "" && len(app.Stages) == 0 {
			delete(f.Apps, name)
			continue
		}
		f.Apps[name] = app
	}
	if len(f.Apps) == 0 {
		f.Apps = nil
	}
}

func parsePause(b []byte) (PauseFile, error) {
	b = bytes.TrimSpace(b)
	if len(b) == 0 {
		return PauseFile{}, nil
	}
	var f PauseFile
	if err := yamlx.Unmarshal(b, &f); err != nil {
		return PauseFile{}, fmt.Errorf("parse %s: %w", PausePath, err)
	}
	return f, nil
}

func marshalPause(f PauseFile) ([]byte, error) {
	f.compact()
	if f.Empty() {
		return []byte(pauseHeader), nil
	}
	body, err := yamlx.Marshal(f)
	if err != nil {
		return nil, err
	}
	out := make([]byte, 0, len(pauseHeader)+len(body))
	out = append(out, pauseHeader...)
	out = append(out, body...)
	return out, nil
}

func (s *Service) CurrentPause() PauseFile {
	f, err := s.readPause()
	if err != nil {
		return PauseFile{}
	}
	return f
}

func (s *Service) readPause() (PauseFile, error) {
	if s == nil || s.OpsRepo == "" {
		return PauseFile{}, nil
	}
	tree, err := gitwrite.ReadPaths(s.OpsRepo, []string{PausePath})
	if err != nil {
		return PauseFile{}, err
	}
	return parsePause(tree[PausePath])
}

func (s *Service) pauseTree() (render.Tree, error) {
	if s == nil || s.OpsRepo == "" {
		return render.Tree{}, nil
	}
	return gitwrite.ReadPaths(s.OpsRepo, []string{PausePath})
}

func (s *Service) denyIfPaused(name, stage string) error {
	if s == nil || !s.Actor.Automated() {
		return nil
	}
	f, err := s.readPause()
	if err != nil {
		return err
	}
	if hit := f.Hit(name, stage); hit != nil {
		return hit
	}
	return nil
}

// SetPause records a pause selector in the ops repo. Empty name and stage is
// global; name only is that app; stage only is that stage on every app.
func (s *Service) SetPause(ctx context.Context, name, stage, reason string) (Mutation, error) {
	name, stage, reason, err := s.pauseSelector(name, stage, reason)
	if err != nil {
		return Mutation{}, err
	}
	if err := s.syncRepo(ctx); err != nil {
		return Mutation{}, err
	}
	before, err := s.pauseTree()
	if err != nil {
		return Mutation{}, err
	}
	entry := PauseEntry{
		At:     time.Now().UTC().Truncate(time.Second),
		By:     s.pauseBy(),
		Reason: reason,
	}
	return s.commitEdit(ctx, pauseMessage("pause", name, stage), before, func(tree render.Tree) error {
		cur, err := s.readPause()
		if err != nil {
			return err
		}
		cur.set(name, stage, entry)
		b, err := marshalPause(cur)
		if err != nil {
			return err
		}
		tree[PausePath] = b
		return nil
	})
}

// ClearPause lifts one pause selector. No-op if that selector is not set.
func (s *Service) ClearPause(ctx context.Context, name, stage string) (Mutation, error) {
	name, stage, _, err := s.pauseSelector(name, stage, "")
	if err != nil {
		return Mutation{}, err
	}
	if err := s.syncRepo(ctx); err != nil {
		return Mutation{}, err
	}
	cur, err := s.readPause()
	if err != nil {
		return Mutation{}, err
	}
	if !cur.has(name, stage) {
		return Mutation{DryRun: !s.Apply}, nil
	}
	before, err := s.pauseTree()
	if err != nil {
		return Mutation{}, err
	}
	return s.commitEdit(ctx, pauseMessage("unpause", name, stage), before, func(tree render.Tree) error {
		cur, err := s.readPause()
		if err != nil {
			return err
		}
		if !cur.has(name, stage) {
			return nil
		}
		cur.clear(name, stage)
		b, err := marshalPause(cur)
		if err != nil {
			return err
		}
		tree[PausePath] = b
		return nil
	})
}

func (s *Service) pauseSelector(name, stage, reason string) (string, string, string, error) {
	name = strings.TrimSpace(name)
	stage = strings.TrimSpace(stage)
	reason = strings.TrimSpace(reason)
	if len(reason) > maxPauseReason {
		return "", "", "", fmt.Errorf("reason must be %d characters or less", maxPauseReason)
	}
	if name != "" {
		if s == nil || s.Catalog == nil {
			return "", "", "", fmt.Errorf("unknown deployable %q", name)
		}
		d, err := s.Catalog.Get(name)
		if err != nil {
			return "", "", "", err
		}
		if stage != "" {
			if _, err := d.Stage(stage); err != nil {
				return "", "", "", err
			}
		}
		return name, stage, reason, nil
	}
	if stage != "" && !s.knownStage(stage) {
		return "", "", "", fmt.Errorf("unknown stage %q", stage)
	}
	return name, stage, reason, nil
}

func (s *Service) knownStage(name string) bool {
	if s == nil || s.Catalog == nil || name == "" {
		return false
	}
	for _, d := range s.Catalog.List() {
		if _, err := d.Stage(name); err == nil {
			return true
		}
	}
	return false
}

func (s *Service) pauseBy() string {
	if s != nil && s.Actor.Kind != "" {
		name, _ := s.Actor.ident()
		if name != "" {
			return name
		}
	}
	return s.author().Name
}

func pauseMessage(verb, name, stage string) string {
	switch {
	case name != "" && stage != "":
		return verb + " " + name + "/" + stage
	case name != "":
		return verb + " " + name
	case stage != "":
		return verb + " " + stage
	default:
		return verb + " all"
	}
}
