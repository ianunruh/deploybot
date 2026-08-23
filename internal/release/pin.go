package release

import (
	"context"
	"fmt"

	"github.com/ianunruh/deploybot/internal/diffx"
	"github.com/ianunruh/deploybot/internal/image"
	"github.com/ianunruh/deploybot/internal/render"
)

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
