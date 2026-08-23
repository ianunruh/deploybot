package release

import (
	"context"
	"fmt"
	"strings"

	"github.com/ianunruh/deploybot/internal/diffx"
	"github.com/ianunruh/deploybot/internal/gitwrite"
	"github.com/ianunruh/deploybot/internal/render"
	"github.com/ianunruh/deploybot/internal/spec"
)

type reconcilePlan struct {
	d      *spec.Deployable
	stages []string
	before render.Tree
	after  render.Tree
	msg    string
}

func (s *Service) planReconcile(name string, stages []string) (reconcilePlan, error) {
	d, err := s.Catalog.Get(name)
	if err != nil {
		return reconcilePlan{}, err
	}
	stages, err = resolveStages(d, stages)
	if err != nil {
		return reconcilePlan{}, err
	}
	generated, err := render.Render(d)
	if err != nil {
		return reconcilePlan{}, err
	}
	generated, err = render.FilterStages(generated, d, stages)
	if err != nil {
		return reconcilePlan{}, err
	}
	before := render.Tree{}
	if s.OpsRepo != "" {
		before, err = gitwrite.ReadPaths(s.OpsRepo, render.SortedPaths(generated))
		if err != nil {
			return reconcilePlan{}, err
		}
	}
	after, err := render.MergeTrees(before, generated)
	if err != nil {
		return reconcilePlan{}, err
	}
	msg := fmt.Sprintf("reconcile %s", name)
	if len(stages) != len(d.Spec.Stages) {
		msg = fmt.Sprintf("reconcile %s (%s)", name, strings.Join(stages, ", "))
	}
	return reconcilePlan{d: d, stages: stages, before: before, after: after, msg: msg}, nil
}

func (s *Service) DiffReconcile(name string, stages []string) (Mutation, error) {
	plan, err := s.planReconcile(name, stages)
	if err != nil {
		return Mutation{}, err
	}
	return Mutation{
		DryRun: true,
		Diff:   diffx.Trees(plan.before, plan.after),
		Files:  changedPaths(plan.before, plan.after),
	}, nil
}

func (s *Service) Reconcile(ctx context.Context, name string, stages []string) (Mutation, error) {
	plan, err := s.planReconcile(name, stages)
	if err != nil {
		return Mutation{}, err
	}
	return s.mutate(ctx, plan.d, plan.msg, plan.before, func(tree render.Tree) error {
		for p := range tree {
			delete(tree, p)
		}
		for p, b := range plan.after {
			tree[p] = b
		}
		return nil
	}, plan.stages)
}

func resolveStages(d *spec.Deployable, stages []string) ([]string, error) {
	if len(stages) == 0 {
		return d.StageNames(), nil
	}
	seen := make(map[string]struct{}, len(stages))
	out := make([]string, 0, len(stages))
	for _, name := range stages {
		if _, err := d.Stage(name); err != nil {
			return nil, err
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	return out, nil
}
