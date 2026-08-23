package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestLoadArgoStages(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "deploybot.yaml")
	body := []byte(`
argo:
  homelab:
    url: https://argocd.k8s.kcloud.zone/
  prod:
    url: https://argocd.k8s.kcloud.io
    tokenFile: secrets/prod.token
    tokenEnv: DEPLOYBOT_ARGO_TOKEN_PROD
`)
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]Argo{
		"homelab": {URL: "https://argocd.k8s.kcloud.zone"},
		"prod": {
			URL:       "https://argocd.k8s.kcloud.io",
			TokenFile: "secrets/prod.token",
			TokenEnv:  "DEPLOYBOT_ARGO_TOKEN_PROD",
		},
	}
	if diff := cmp.Diff(want, f.Argo); diff != "" {
		t.Fatal(diff)
	}
}

func TestLoadRejectsEmptyStage(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "deploybot.yaml")
	if err := os.WriteFile(path, []byte("argo:\n  \"\":\n    url: https://x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadBadYAML(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "deploybot.yaml")
	if err := os.WriteFile(path, []byte(":\n  - [\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected error")
	}
}

func TestResolvePathExplicitAndEnv(t *testing.T) {
	t.Setenv("DEPLOYBOT_CONFIG", "from-env.yaml")
	got, err := ResolvePath("from-flag.yaml")
	if err != nil || got != "from-flag.yaml" {
		t.Fatalf("explicit %q %v", got, err)
	}
	got, err = ResolvePath("")
	if err != nil || got != "from-env.yaml" {
		t.Fatalf("env %q %v", got, err)
	}
}

func TestResolvePathDefaultFile(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("DEPLOYBOT_CONFIG", "")
	got, err := ResolvePath("")
	if err != nil || got != "" {
		t.Fatalf("missing default %q %v", got, err)
	}
	if err := os.WriteFile(DefaultPath, []byte("addr: 127.0.0.1:9\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err = ResolvePath("")
	if err != nil || got != DefaultPath {
		t.Fatalf("default %q %v", got, err)
	}
	f, path, err := Open("")
	if err != nil || path != DefaultPath || f.Addr != "127.0.0.1:9" {
		t.Fatalf("open %+v %q %v", f, path, err)
	}
}

func TestOpenMissingExplicit(t *testing.T) {
	t.Parallel()
	if _, _, err := Open(filepath.Join(t.TempDir(), "nope.yaml")); err == nil {
		t.Fatal("expected error")
	}
}

func TestEnvBool(t *testing.T) {
	t.Setenv("DEPLOYBOT_APPLY", "1")
	v, set := EnvBool("DEPLOYBOT_APPLY")
	if !v || !set {
		t.Fatalf("1: %v %v", v, set)
	}
	t.Setenv("DEPLOYBOT_APPLY", "0")
	v, set = EnvBool("DEPLOYBOT_APPLY")
	if v || !set {
		t.Fatalf("0: %v %v", v, set)
	}
	t.Setenv("DEPLOYBOT_APPLY", "")
	v, set = EnvBool("DEPLOYBOT_APPLY")
	if v || set {
		t.Fatalf("empty: %v %v", v, set)
	}
}
