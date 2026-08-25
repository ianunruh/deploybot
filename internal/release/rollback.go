package release

import (
	"context"
	"fmt"

	"github.com/ianunruh/deploybot/internal/image"
	"github.com/ianunruh/deploybot/internal/render"
	"github.com/ianunruh/deploybot/internal/spec"
)

func (s *Service) Rollback(ctx context.Context, name, stage, imageRef string) (Mutation, error) {
	d, err := s.Catalog.Get(name)
	if err != nil {
		return Mutation{}, err
	}
	if _, err := d.Stage(stage); err != nil {
		return Mutation{}, err
	}
	want, err := image.Parse(imageRef)
	if err != nil {
		return Mutation{}, err
	}
	if err := s.syncRepo(ctx); err != nil {
		return Mutation{}, err
	}
	tree, err := s.overlayTree(ctx, d)
	if err != nil {
		return Mutation{}, err
	}
	ref, err := s.resolveRollback(ctx, d, tree, stage, want)
	if err != nil {
		return Mutation{}, err
	}
	return s.mutate(ctx, d, fmt.Sprintf("rollback %s %s to %s", name, stage, ref.LogName()), tree, func(tree render.Tree) error {
		return render.Pin(tree, d, stage, ref)
	}, []string{stage})
}

func (s *Service) resolveRollback(ctx context.Context, d *spec.Deployable, tree render.Tree, stage string, want image.Ref) (image.Ref, error) {
	current, err := render.CurrentImage(tree, d, stage)
	if err == nil && current.SameRelease(want) {
		return image.Ref{}, fmt.Errorf("%s %s is already at %s", d.Metadata.Name, stage, want.LogName())
	}
	events, err := s.overlayChanges(ctx, d, defaultHistoryLimit)
	if err != nil {
		return image.Ref{}, err
	}
	repo := d.Spec.Image.Repository
	for _, e := range events {
		if e.Stage != stage {
			continue
		}
		got := eventRef(repo, e)
		if !historicRelease(got, want) {
			continue
		}
		if want.Tag == "" {
			want.Tag = got.Tag
		}
		if want.Digest == "" {
			want.Digest = got.Digest
		}
		if want.Repository == "" {
			want.Repository = repo
		}
		return want, nil
	}
	return image.Ref{}, fmt.Errorf("no previous pin of %s on %s", want.LogName(), stage)
}

func historicRelease(got, want image.Ref) bool {
	if got.SameRelease(want) {
		return true
	}
	if want.Digest != "" && got.Digest == want.Digest {
		return true
	}
	return want.Digest == "" && want.Tag != "" && got.Tag == want.Tag
}

func eventRef(repo string, e Event) image.Ref {
	return image.Ref{Repository: repo, Tag: e.Tag, Digest: e.Digest}
}
