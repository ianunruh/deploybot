package render

import (
	"path"
	"strings"

	"github.com/ianunruh/deploybot/internal/spec"
)

// FilterStages keeps shared workload base files plus overlay files for the
// given stages. Empty stages means every stage. Unknown stage names error.
func FilterStages(tree Tree, d *spec.Deployable, stages []string) (Tree, error) {
	if len(stages) == 0 {
		stages = d.StageNames()
	}
	want := make(map[string]struct{}, len(stages))
	for _, name := range stages {
		if _, err := d.Stage(name); err != nil {
			return nil, err
		}
		want[name] = struct{}{}
	}
	out := Tree{}
	for p, b := range tree {
		if keepStagePath(d, p, want) {
			out[p] = b
		}
	}
	return out, nil
}

func keepStagePath(d *spec.Deployable, p string, stages map[string]struct{}) bool {
	p = path.Clean(p)
	workload := path.Clean(d.Spec.Git.WorkloadPath)
	if inDir(p, path.Join(workload, "base")) {
		return true
	}
	for st := range stages {
		if inDir(p, path.Join(workload, "overlays", st)) {
			return true
		}
		if inDir(p, path.Join(path.Clean(d.Spec.Git.ApplicationPath), "overlays", st)) {
			return true
		}
	}
	return false
}

func inDir(p, dir string) bool {
	return p == dir || strings.HasPrefix(p, dir+"/")
}
