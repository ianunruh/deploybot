package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func isolateEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DEPLOYBOT_CONFIG", "")
	t.Setenv("DEPLOYBOT_APPLY", "")
	t.Setenv("DEPLOYBOT_PUSH", "")
	t.Setenv("DEPLOYBOT_SYNC", "")
	t.Setenv("DEPLOYBOT_OPS_REPO", "")
	t.Setenv("DEPLOYBOT_OPS_REPO_URL", "")
	t.Setenv("DEPLOYBOT_ADDR", "")
	t.Setenv("DEPLOYBOT_SPECS_DIR", "")
	t.Setenv("DEPLOYBOT_ARGO_URL", "")
	t.Setenv("KUBECONFIG", filepath.Join(t.TempDir(), "kubeconfig"))
}

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

func TestRunReconcile(t *testing.T) {
	isolateEnv(t)
	spec := filepath.Join("..", "..", "examples", "kmc.yaml")
	if err := Run(t.Context(), []string{"reconcile", "--spec", spec, "--stage", "homelab"}); err != nil {
		t.Fatal(err)
	}
}

func TestRunReconcileUnknownStage(t *testing.T) {
	isolateEnv(t)
	spec := filepath.Join("..", "..", "examples", "kmc.yaml")
	if err := Run(t.Context(), []string{"reconcile", "--spec", spec, "--stage", "nope"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestRunPushRequiresApply(t *testing.T) {
	isolateEnv(t)
	spec := filepath.Join("..", "..", "examples", "kmc.yaml")
	err := Run(t.Context(), []string{
		"pin", "--spec", spec, "--stage", "homelab",
		"--image", "ghcr.io/ianunruh/kmc:x", "--push",
	})
	if err == nil || err.Error() != "--push requires --apply" {
		t.Fatalf("got %v", err)
	}
}

func TestRunMissingConfig(t *testing.T) {
	isolateEnv(t)
	spec := filepath.Join("..", "..", "examples", "kmc.yaml")
	err := Run(t.Context(), []string{
		"reconcile", "--spec", spec, "--stage", "homelab",
		"--config", filepath.Join(t.TempDir(), "nope.yaml"),
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRunConfigArgoYAML(t *testing.T) {
	isolateEnv(t)
	dir := t.TempDir()
	cfg := filepath.Join(dir, "deploybot.yaml")
	body := []byte("clusters:\n  homelab:\n    argo:\n      url: https://argocd.k8s.kcloud.zone\n")
	if err := os.WriteFile(cfg, body, 0o644); err != nil {
		t.Fatal(err)
	}
	spec := filepath.Join("..", "..", "examples", "kmc.yaml")
	if err := Run(t.Context(), []string{
		"reconcile", "--spec", spec, "--stage", "homelab", "--config", cfg,
	}); err != nil {
		t.Fatal(err)
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
