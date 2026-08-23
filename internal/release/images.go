package release

import (
	"context"
	"fmt"

	"github.com/ianunruh/deploybot/internal/image"
)

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
