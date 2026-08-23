package render

import (
	"strings"
	"testing"
)

func TestFilterStagesOmitsOtherOverlays(t *testing.T) {
	t.Parallel()
	d := loadExample(t, "kmc")
	tree, err := Render(d)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := tree["k8s/apps/projects/sandbox/base/kmc.yaml"]; ok {
		t.Fatal("Application YAML must live in stage overlays, not project base")
	}
	filtered, err := FilterStages(tree, d, []string{"homelab"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"k8s/kmc/base/deployment.yaml",
		"k8s/kmc/overlays/homelab/kustomization.yaml",
		"k8s/apps/projects/sandbox/overlays/homelab/kmc.yaml",
	}
	for _, p := range want {
		if _, ok := filtered[p]; !ok {
			t.Fatalf("missing %s", p)
		}
	}
	for _, p := range []string{
		"k8s/kmc/overlays/prod/kustomization.yaml",
		"k8s/kmc/overlays/prod/patch-httproute.yaml",
		"k8s/apps/projects/sandbox/overlays/prod/kmc.yaml",
	} {
		if _, ok := filtered[p]; ok {
			t.Fatalf("homelab filter kept %s", p)
		}
	}
	app := string(filtered["k8s/apps/projects/sandbox/overlays/homelab/kmc.yaml"])
	if !strings.Contains(app, "path: k8s/kmc/overlays/homelab") {
		t.Fatalf("homelab Application should be complete:\n%s", app)
	}
	if !strings.Contains(app, "project: sandbox") {
		t.Fatalf("homelab Application missing project:\n%s", app)
	}
	if _, err := FilterStages(tree, d, []string{"nope"}); err == nil {
		t.Fatal("expected unknown stage")
	}
}
