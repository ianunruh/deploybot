package release

import (
	"context"
	"fmt"

	"github.com/ianunruh/deploybot/internal/render"
)

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
