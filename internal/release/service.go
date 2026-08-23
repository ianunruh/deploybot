package release

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ianunruh/deploybot/internal/argo"
	"github.com/ianunruh/deploybot/internal/catalog"
	"github.com/ianunruh/deploybot/internal/diffx"
	"github.com/ianunruh/deploybot/internal/gitwrite"
	"github.com/ianunruh/deploybot/internal/image"
	"github.com/ianunruh/deploybot/internal/render"
	"github.com/ianunruh/deploybot/internal/spec"
)

type Service struct {
	Catalog *catalog.Catalog
	OpsRepo string
	Apply   bool
	Sync    bool
	Author  gitwrite.Author
	Argo    argo.Router
	Wait    time.Duration
	Images  image.Lister
}

type StageStatus struct {
	Name     string `json:"name"`
	Hostname string `json:"hostname"`
	Image    string `json:"image,omitempty"`
	Sync     string `json:"sync"`
	Health   string `json:"health"`
	Revision string `json:"revision,omitempty"`
	Message  string `json:"message,omitempty"`
}

type Status struct {
	Name      string        `json:"name"`
	Namespace string        `json:"namespace"`
	ImageRepo string        `json:"imageRepo"`
	Stages    []StageStatus `json:"stages"`
	Apply     bool          `json:"apply"`
}

type Mutation struct {
	DryRun bool     `json:"dryRun"`
	Commit string   `json:"commit,omitempty"`
	Diff   string   `json:"diff"`
	Files  []string `json:"files"`
	Synced bool     `json:"synced"`
}

func (s *Service) Status(ctx context.Context, name string) (Status, error) {
	d, err := s.Catalog.Get(name)
	if err != nil {
		return Status{}, err
	}
	tree, err := s.workingTree(d)
	if err != nil {
		return Status{}, err
	}
	out := Status{
		Name:      d.Metadata.Name,
		Namespace: d.Spec.Namespace,
		ImageRepo: d.Spec.Image.Repository,
		Apply:     s.Apply,
	}
	for _, st := range d.Spec.Stages {
		ss := StageStatus{
			Name:     st.Name,
			Hostname: st.Hostname,
			Sync:     "unknown",
			Health:   "unknown",
		}
		if img, err := render.CurrentImage(tree, d, st.Name); err == nil {
			ss.Image = img.Compact()
		}
		if s.Argo != nil {
			if c := s.Argo.ForStage(st.Name); c != nil {
				got, err := c.Get(ctx, d.Spec.Argo.Name)
				if err != nil {
					ss.Message = err.Error()
				} else {
					ss.Health = got.Health
					ss.Sync = got.Sync
					ss.Revision = got.Revision
					ss.Message = got.Message
				}
			}
		}
		out.Stages = append(out.Stages, ss)
	}
	return out, nil
}

type ImageList struct {
	Repository string          `json:"repository"`
	Source     string          `json:"source"`
	Images     []image.Version `json:"images"`
}

func (s *Service) ListImages(ctx context.Context, name string) (ImageList, error) {
	d, err := s.Catalog.Get(name)
	if err != nil {
		return ImageList{}, err
	}
	if s.Images == nil {
		return ImageList{}, fmt.Errorf("image listing is not configured")
	}
	listing, err := s.Images.List(ctx, d.Spec.Image.Repository, d.Spec.Image.Tag)
	if err != nil {
		return ImageList{}, err
	}
	images := listing.Versions
	if images == nil {
		images = []image.Version{}
	}
	return ImageList{
		Repository: d.Spec.Image.Repository,
		Source:     listing.Source,
		Images:     images,
	}, nil
}

func (s *Service) Pin(ctx context.Context, name, stage, imageRef string) (Mutation, error) {
	d, err := s.Catalog.Get(name)
	if err != nil {
		return Mutation{}, err
	}
	if _, err := d.Stage(stage); err != nil {
		return Mutation{}, err
	}
	ref, err := image.Parse(imageRef)
	if err != nil {
		return Mutation{}, err
	}
	tree, err := s.overlayTree(d)
	if err != nil {
		return Mutation{}, err
	}
	return s.mutate(ctx, d, fmt.Sprintf("pin %s %s to %s", name, stage, ref.LogName()), tree, func(tree render.Tree) error {
		return render.Pin(tree, d, stage, ref)
	}, []string{stage})
}

func (s *Service) Promote(ctx context.Context, name, from, to string) (Mutation, error) {
	d, err := s.Catalog.Get(name)
	if err != nil {
		return Mutation{}, err
	}
	if _, err := d.Stage(from); err != nil {
		return Mutation{}, err
	}
	if _, err := d.Stage(to); err != nil {
		return Mutation{}, err
	}
	if s.Argo != nil && s.Argo.ForStage(from) != nil {
		if err := s.waitStage(ctx, d, from); err != nil {
			return Mutation{}, fmt.Errorf("health gate %s: %w", from, err)
		}
	}
	tree, err := s.workingTree(d)
	if err != nil {
		return Mutation{}, err
	}
	img, err := render.CurrentImage(tree, d, from)
	if err != nil {
		return Mutation{}, fmt.Errorf("source stage %s: %w", from, err)
	}
	return s.mutate(ctx, d, fmt.Sprintf("promote %s %s -> %s (%s)", name, from, to, img.LogName()), tree, func(tree render.Tree) error {
		return render.Pin(tree, d, to, img)
	}, []string{to})
}

type syncPlan struct {
	d      *spec.Deployable
	stages []string
	before render.Tree
	after  render.Tree
	msg    string
}

func (s *Service) planSync(name string, stages []string) (syncPlan, error) {
	d, err := s.Catalog.Get(name)
	if err != nil {
		return syncPlan{}, err
	}
	stages, err = resolveStages(d, stages)
	if err != nil {
		return syncPlan{}, err
	}
	generated, err := render.Render(d)
	if err != nil {
		return syncPlan{}, err
	}
	generated, err = render.FilterStages(generated, d, stages)
	if err != nil {
		return syncPlan{}, err
	}
	before := render.Tree{}
	if s.OpsRepo != "" {
		before, err = gitwrite.ReadPaths(s.OpsRepo, render.SortedPaths(generated))
		if err != nil {
			return syncPlan{}, err
		}
	}
	after, err := render.MergeTrees(before, generated)
	if err != nil {
		return syncPlan{}, err
	}
	msg := fmt.Sprintf("sync %s", name)
	if len(stages) != len(d.Spec.Stages) {
		msg = fmt.Sprintf("sync %s (%s)", name, strings.Join(stages, ", "))
	}
	return syncPlan{d: d, stages: stages, before: before, after: after, msg: msg}, nil
}

func (s *Service) DiffSync(name string, stages []string) (Mutation, error) {
	plan, err := s.planSync(name, stages)
	if err != nil {
		return Mutation{}, err
	}
	return Mutation{
		DryRun: true,
		Diff:   diffx.Trees(plan.before, plan.after),
		Files:  changedPaths(plan.before, plan.after),
	}, nil
}

func (s *Service) SyncManifests(ctx context.Context, name string, stages []string) (Mutation, error) {
	plan, err := s.planSync(name, stages)
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

func (s *Service) Diff(name, stage, imageRef string) (string, error) {
	d, err := s.Catalog.Get(name)
	if err != nil {
		return "", err
	}
	ref, err := image.Parse(imageRef)
	if err != nil {
		return "", err
	}
	before, err := s.overlayTree(d)
	if err != nil {
		return "", err
	}
	after, err := cloneTree(before)
	if err != nil {
		return "", err
	}
	if err := render.Pin(after, d, stage, ref); err != nil {
		return "", err
	}
	return diffx.Trees(before, after), nil
}

func (s *Service) mutate(ctx context.Context, d *spec.Deployable, message string, before render.Tree, edit func(render.Tree) error, syncStages []string) (Mutation, error) {
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

func (s *Service) workingTree(d *spec.Deployable) (render.Tree, error) {
	return s.overlayTree(d)
}

// overlayTree is the stage overlay kustomizations only. Pin/promote must not
// rewrite workload YAML or shared Argo project kustomizations.
func (s *Service) overlayTree(d *spec.Deployable) (render.Tree, error) {
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

func (s *Service) author() gitwrite.Author {
	if s.Author.Name != "" {
		return s.Author
	}
	return gitwrite.DefaultAuthor()
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
