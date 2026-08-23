package release

import (
	"context"
	"fmt"
	"time"

	"github.com/ianunruh/deploybot/internal/argo"
	"github.com/ianunruh/deploybot/internal/diffx"
	"github.com/ianunruh/deploybot/internal/gitwrite"
	"github.com/ianunruh/deploybot/internal/render"
	"github.com/ianunruh/deploybot/internal/spec"
)

func (s *Service) mutate(ctx context.Context, d *spec.Deployable, message string, before render.Tree, edit func(render.Tree) error, syncStages []string) (Mutation, error) {
	if s.Push && !s.Apply {
		return Mutation{}, fmt.Errorf("push requires apply")
	}
	after, err := cloneTree(before)
	if err != nil {
		return Mutation{}, err
	}
	if err := edit(after); err != nil {
		return Mutation{}, err
	}
	mut := Mutation{
		DryRun: !s.Apply,
		Diff:   diffx.Trees(before, after),
		Files:  changedPaths(before, after),
	}
	if !s.Apply {
		return mut, nil
	}
	if s.OpsRepo == "" {
		return Mutation{}, fmt.Errorf("DEPLOYBOT_OPS_REPO is required to apply")
	}
	toWrite := render.Tree{}
	for _, p := range mut.Files {
		toWrite[p] = after[p]
	}
	res, err := gitwrite.Write(s.OpsRepo, toWrite, message, s.author())
	if err != nil {
		return Mutation{}, err
	}
	mut.Commit = res.Commit
	if s.Push {
		pushed, err := gitwrite.Push(ctx, s.OpsRepo)
		if err != nil {
			return mut, err
		}
		mut.Pushed = true
		mut.Ref = pushed.Ref()
	}
	if s.Sync {
		for _, st := range syncStages {
			if err := s.syncStage(ctx, d, st); err != nil {
				return mut, err
			}
			if err := s.waitStage(ctx, d, st); err != nil {
				return mut, err
			}
		}
		mut.Synced = len(syncStages) > 0
	}
	return mut, nil
}

func (s *Service) workingTree(ctx context.Context, d *spec.Deployable) (render.Tree, error) {
	return s.overlayTree(ctx, d)
}

func (s *Service) syncRepo(ctx context.Context) error {
	if s.OpsRepo == "" {
		return nil
	}
	return gitwrite.Pull(ctx, s.OpsRepo)
}

// overlayTree is the stage overlay kustomizations only. Pin/promote must not
// rewrite workload YAML or shared Argo project kustomizations.
func (s *Service) overlayTree(ctx context.Context, d *spec.Deployable) (render.Tree, error) {
	if err := s.syncRepo(ctx); err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(d.Spec.Stages))
	for _, st := range d.Spec.Stages {
		paths = append(paths, render.OverlayKustomizationPath(d, st.Name))
	}
	out := render.Tree{}
	if s.OpsRepo != "" {
		existing, err := gitwrite.ReadPaths(s.OpsRepo, paths)
		if err != nil {
			return nil, err
		}
		for p, b := range existing {
			out[p] = b
		}
	}
	if len(out) == len(paths) {
		return out, nil
	}
	generated, err := render.Render(d)
	if err != nil {
		return nil, err
	}
	for _, p := range paths {
		if _, ok := out[p]; ok {
			continue
		}
		out[p] = generated[p]
	}
	return out, nil
}

func (s *Service) syncStage(ctx context.Context, d *spec.Deployable, stage string) error {
	if s.Argo == nil {
		return fmt.Errorf("no Argo endpoint for stage %s", stage)
	}
	c := s.Argo.ForStage(stage)
	if c == nil {
		return fmt.Errorf("no Argo endpoint for stage %s", stage)
	}
	return c.Sync(ctx, d.Spec.Argo.Name, true)
}

func (s *Service) waitStage(ctx context.Context, d *spec.Deployable, stage string) error {
	if s.Argo == nil {
		return nil
	}
	c := s.Argo.ForStage(stage)
	if c == nil {
		return nil
	}
	wait := s.Wait
	if wait == 0 {
		wait = 5 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, wait)
	defer cancel()
	return argo.WaitHealthy(ctx, c, d.Spec.Argo.Name, 2*time.Second)
}

func cloneTree(t render.Tree) (render.Tree, error) {
	out := make(render.Tree, len(t))
	for p, b := range t {
		cp := make([]byte, len(b))
		copy(cp, b)
		out[p] = cp
	}
	return out, nil
}

func changedPaths(before, after render.Tree) []string {
	var paths []string
	for _, p := range render.SortedPaths(after) {
		if string(before[p]) != string(after[p]) {
			paths = append(paths, p)
		}
	}
	return paths
}
