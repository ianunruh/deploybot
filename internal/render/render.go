package render

import (
	"path"
	"slices"

	"github.com/ianunruh/deploybot/internal/spec"
)

const (
	labelName      = "app.kubernetes.io/name"
	labelManagedBy = "app.kubernetes.io/managed-by"
	managedBy      = "deploybot"
)

// Tree is a map of repo-relative paths to file contents.
type Tree map[string][]byte

func Render(d *spec.Deployable) (Tree, error) {
	out := make(Tree)
	if err := writeWorkload(out, d); err != nil {
		return nil, err
	}
	if err := writeArgo(out, d); err != nil {
		return nil, err
	}
	return out, nil
}

func overlayPath(d *spec.Deployable, stage string) string {
	return path.Join(d.Spec.Git.WorkloadPath, "overlays", stage)
}

func OverlayKustomizationPath(d *spec.Deployable, stage string) string {
	return path.Join(overlayPath(d, stage), "kustomization.yaml")
}

func ApplicationOverlayPath(d *spec.Deployable, stage string) string {
	return path.Join(d.Spec.Git.ApplicationPath, "overlays", stage, d.Spec.Argo.Name+".yaml")
}

func labels(d *spec.Deployable) map[string]string {
	return map[string]string{
		labelName:      d.Metadata.Name,
		labelManagedBy: managedBy,
	}
}

func cmpOr[T comparable](v, fallback T) T {
	var zero T
	if v == zero {
		return fallback
	}
	return v
}

func SortedPaths(t Tree) []string {
	paths := make([]string, 0, len(t))
	for p := range t {
		paths = append(paths, p)
	}
	slices.Sort(paths)
	return paths
}
