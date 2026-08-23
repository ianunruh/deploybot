package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunVersion(t *testing.T) {
	t.Parallel()
	if err := Run(t.Context(), []string{"version"}); err != nil {
		t.Fatal(err)
	}
}

func TestRunUnknown(t *testing.T) {
	t.Parallel()
	if err := Run(t.Context(), []string{"nope"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestRunRender(t *testing.T) {
	t.Parallel()
	out := t.TempDir()
	spec := filepath.Join("..", "..", "examples", "kmc.yaml")
	if _, err := os.Stat(spec); err != nil {
		t.Fatal(err)
	}
	if err := Run(t.Context(), []string{"render", "--out", out, spec}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(out, "k8s/kmc/base/deployment.yaml")); err != nil {
		t.Fatal(err)
	}
}
