package render

import (
	"fmt"
	"slices"
	"strings"

	"github.com/ianunruh/deploybot/internal/yamlx"
)

// MergeTrees keeps extra files from existing and merges kustomization.yaml
// files so human-owned generators/patches survive a pin.
func MergeTrees(existing, generated Tree) (Tree, error) {
	out := make(Tree, len(existing)+len(generated))
	for p, b := range existing {
		out[p] = slices.Clone(b)
	}
	for p, gen := range generated {
		cur, ok := out[p]
		if !ok || !strings.HasSuffix(p, "/kustomization.yaml") && p != "kustomization.yaml" {
			out[p] = gen
			continue
		}
		merged, err := mergeKustomization(cur, gen)
		if err != nil {
			return nil, fmt.Errorf("merge %s: %w", p, err)
		}
		out[p] = merged
	}
	return out, nil
}

func mergeKustomization(existing, generated []byte) ([]byte, error) {
	var e, g kustomization
	if len(existing) > 0 {
		if err := yamlx.Unmarshal(existing, &e); err != nil {
			return nil, err
		}
	}
	if err := yamlx.Unmarshal(generated, &g); err != nil {
		return nil, err
	}
	out := e
	out.APIVersion = cmpOr(out.APIVersion, g.APIVersion)
	out.Kind = cmpOr(out.Kind, g.Kind)
	if g.Namespace != "" {
		out.Namespace = g.Namespace
	}
	out.Resources = unionStable(out.Resources, g.Resources)
	for _, img := range g.Images {
		out.Images = upsertImage(out.Images, img)
	}
	out.Patches = unionPatches(out.Patches, g.Patches)
	if len(g.Labels) > 0 && len(out.Labels) == 0 {
		out.Labels = g.Labels
	}
	return yamlx.MarshalGenerated(out)
}

func upsertImage(images []kustomizeImage, img kustomizeImage) []kustomizeImage {
	for i, existing := range images {
		if existing.Name == img.Name {
			images[i] = img
			return images
		}
	}
	return append(images, img)
}

func unionStable(a, b []string) []string {
	seen := make(map[string]struct{}, len(a)+len(b))
	var out []string
	for _, s := range append(slices.Clone(a), b...) {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func unionPatches(a, b []kustomizePatch) []kustomizePatch {
	seen := make(map[string]struct{}, len(a)+len(b))
	var out []kustomizePatch
	for _, p := range append(slices.Clone(a), b...) {
		key := p.Path
		if p.Target != nil {
			key += "|" + p.Target.Kind + "|" + p.Target.Name
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, p)
	}
	return out
}
