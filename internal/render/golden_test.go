package render

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ianunruh/deploybot/internal/image"
	"github.com/ianunruh/deploybot/internal/spec"
	"github.com/ianunruh/deploybot/internal/yamlx"
)

func TestKMCGolden(t *testing.T) {
	t.Parallel()
	d := loadKMC(t)
	tree, err := Render(d)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(testDataDir(t), "kmc")
	if os.Getenv("UPDATE_GOLDENS") == "1" {
		if err := writeTree(dir, tree); err != nil {
			t.Fatal(err)
		}
	}
	want, err := readTree(dir)
	if err != nil {
		t.Fatalf("read goldens (run UPDATE_GOLDENS=1 go test): %v", err)
	}
	if diff := cmp.Diff(stringMap(want), stringMap(tree)); diff != "" {
		t.Fatalf("golden mismatch (-want +got):\n%s", diff)
	}
}

func TestPinIsOnlyOverlayImageDiff(t *testing.T) {
	t.Parallel()
	d := loadKMC(t)
	before, err := Render(d)
	if err != nil {
		t.Fatal(err)
	}
	after, err := Render(d)
	if err != nil {
		t.Fatal(err)
	}
	ref := image.MustParse("ghcr.io/ianunruh/kmc:main-abc123@sha256:deadbeef")
	if err := Pin(after, d, "homelab", ref); err != nil {
		t.Fatal(err)
	}
	changed := 0
	for _, p := range SortedPaths(after) {
		if string(before[p]) == string(after[p]) {
			continue
		}
		changed++
		if p != OverlayKustomizationPath(d, "homelab") {
			t.Fatalf("pin changed unexpected file %s", p)
		}
	}
	if changed != 1 {
		t.Fatalf("pin changed %d files, want 1", changed)
	}
	got, err := CurrentImage(after, d, "homelab")
	if err != nil {
		t.Fatal(err)
	}
	if got != ref {
		t.Fatalf("current image %+v want %+v", got, ref)
	}
}

func TestMergeKeepsConfigMapGenerator(t *testing.T) {
	t.Parallel()
	d := loadKMC(t)
	generated, err := Render(d)
	if err != nil {
		t.Fatal(err)
	}
	overlay := OverlayKustomizationPath(d, "prod")
	existing := Tree{
		overlay: []byte(`apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - ../../base
configMapGenerator:
  - name: kmc-env
    behavior: replace
    envs:
      - config.env
`),
	}
	merged, err := MergeTrees(existing, generated)
	if err != nil {
		t.Fatal(err)
	}
	var k kustomization
	if err := yamlx.Unmarshal(merged[overlay], &k); err != nil {
		t.Fatal(err)
	}
	if len(k.ConfigMapGenerator) != 1 || k.ConfigMapGenerator[0].Name != "kmc-env" {
		t.Fatalf("lost configMapGenerator: %+v", k.ConfigMapGenerator)
	}
}

func loadKMC(t *testing.T) *spec.Deployable {
	t.Helper()
	d, err := spec.Load(filepath.Join(repoRoot(t), "examples/kmc.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func testDataDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(filepath.Dir(funcFile()), "testdata")
}

func funcFile() string {
	_, file, _, _ := runtime.Caller(0)
	return file
}

func stringMap(t Tree) map[string]string {
	out := make(map[string]string, len(t))
	for p, b := range t {
		out[p] = string(b)
	}
	return out
}

func writeTree(dir string, tree Tree) error {
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	for p, b := range tree {
		fp := filepath.Join(dir, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(fp), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(fp, b, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func readTree(dir string) (Tree, error) {
	out := Tree{}
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		out[filepath.ToSlash(rel)] = b
		return nil
	})
	return out, err
}
