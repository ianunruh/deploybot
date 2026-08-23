package render

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ianunruh/deploybot/internal/image"
	"github.com/ianunruh/deploybot/internal/spec"
	"github.com/ianunruh/deploybot/internal/yamlx"
)

func TestGoldens(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"kmc", "kmc-controller"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			d := loadExample(t, name)
			tree, err := Render(d)
			if err != nil {
				t.Fatal(err)
			}
			dir := filepath.Join(testDataDir(t), name)
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
		})
	}
}

func TestYAMLProbePort(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, want string
	}{
		{"8081", "port: 8081\n"},
		{"http", "port: http\n"},
	}
	for _, tc := range cases {
		b, err := yamlx.Marshal(httpGet{Path: "/", Port: yamlProbePort(tc.in)})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(b), tc.want) || strings.Contains(string(b), `port: "8081"`) {
			t.Fatalf("port %q marshaled as:\n%s", tc.in, b)
		}
	}
}

func TestPinIsOnlyOverlayImageDiff(t *testing.T) {
	t.Parallel()
	d := loadExample(t, "kmc")
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

func TestControllerOmitsServiceAndRoute(t *testing.T) {
	t.Parallel()
	d := loadExample(t, "kmc-controller")
	tree, err := Render(d)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{
		"k8s/kmc-controller/base/service.yaml",
		"k8s/kmc-controller/base/httproute.yaml",
		"k8s/kmc-controller/overlays/prod/patch-httproute.yaml",
	} {
		if _, ok := tree[p]; ok {
			t.Fatalf("controller render emitted %s", p)
		}
	}
	kust := string(tree["k8s/kmc-controller/base/kustomization.yaml"])
	if strings.Contains(kust, "service.yaml") || strings.Contains(kust, "httproute.yaml") {
		t.Fatalf("base kustomization should only list deployment.yaml:\n%s", kust)
	}
	dep := string(tree["k8s/kmc-controller/base/deployment.yaml"])
	if strings.Contains(dep, "containerPort") {
		t.Fatalf("generated deployment should omit ports:\n%s", dep)
	}
	if !strings.Contains(dep, "path: /healthz") || !strings.Contains(dep, "path: /readyz") {
		t.Fatalf("generated deployment missing probes:\n%s", dep)
	}
	if strings.Contains(dep, `port: "8081"`) {
		t.Fatalf("numeric probe ports must marshal as integers, not quoted strings:\n%s", dep)
	}
	if !strings.Contains(dep, "port: 8081") {
		t.Fatalf("generated deployment missing numeric probe port:\n%s", dep)
	}
}

func TestMergeKeepsControllerHumanFiles(t *testing.T) {
	t.Parallel()
	d := loadExample(t, "kmc-controller")
	generated, err := Render(d)
	if err != nil {
		t.Fatal(err)
	}
	existing := Tree{
		"k8s/kmc-controller/base/kustomization.yaml": []byte(`apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - deployment.yaml
  - crds/ipaddresses.yaml
  - rbac/service_account.yaml
patches:
  - path: patch-manager.yaml
`),
		"k8s/kmc-controller/base/crds/ipaddresses.yaml":     []byte("kind: CustomResourceDefinition\n"),
		"k8s/kmc-controller/base/rbac/service_account.yaml": []byte("kind: ServiceAccount\n"),
		"k8s/kmc-controller/base/patch-manager.yaml":        []byte("kind: Deployment\n"),
		"k8s/kmc-controller/overlays/homelab/kustomization.yaml": []byte(`apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - ../../base
patches:
  - path: patch-cidrs.yaml
`),
		"k8s/kmc-controller/overlays/homelab/patch-cidrs.yaml": []byte("kind: Deployment\n"),
	}
	merged, err := MergeTrees(existing, generated)
	if err != nil {
		t.Fatal(err)
	}
	var base kustomization
	if err := yamlx.Unmarshal(merged["k8s/kmc-controller/base/kustomization.yaml"], &base); err != nil {
		t.Fatal(err)
	}
	if !containsAll(base.Resources, "deployment.yaml", "crds/ipaddresses.yaml", "rbac/service_account.yaml") {
		t.Fatalf("base resources %+v", base.Resources)
	}
	if len(base.Patches) != 1 || base.Patches[0].Path != "patch-manager.yaml" {
		t.Fatalf("base patches %+v", base.Patches)
	}
	var overlay kustomization
	if err := yamlx.Unmarshal(merged[OverlayKustomizationPath(d, "homelab")], &overlay); err != nil {
		t.Fatal(err)
	}
	if len(overlay.Patches) != 1 || overlay.Patches[0].Path != "patch-cidrs.yaml" {
		t.Fatalf("overlay patches %+v", overlay.Patches)
	}
	for _, p := range []string{
		"k8s/kmc-controller/base/crds/ipaddresses.yaml",
		"k8s/kmc-controller/base/rbac/service_account.yaml",
		"k8s/kmc-controller/base/patch-manager.yaml",
		"k8s/kmc-controller/overlays/homelab/patch-cidrs.yaml",
	} {
		if string(merged[p]) != string(existing[p]) {
			t.Fatalf("lost extra file %s", p)
		}
	}

	ref := image.MustParse("ghcr.io/ianunruh/kmc-controller:main-abc@sha256:deadbeef")
	if err := Pin(merged, d, "homelab", ref); err != nil {
		t.Fatal(err)
	}
	if string(merged["k8s/kmc-controller/overlays/homelab/patch-cidrs.yaml"]) != string(existing["k8s/kmc-controller/overlays/homelab/patch-cidrs.yaml"]) {
		t.Fatal("pin rewrote CIDR overlay")
	}
	got, err := CurrentImage(merged, d, "homelab")
	if err != nil {
		t.Fatal(err)
	}
	if got != ref {
		t.Fatalf("current image %+v want %+v", got, ref)
	}
}

func TestMergeKeepsKMCHumanFiles(t *testing.T) {
	t.Parallel()
	d := loadExample(t, "kmc")
	generated, err := Render(d)
	if err != nil {
		t.Fatal(err)
	}
	existing := Tree{
		"k8s/kmc/base/kustomization.yaml": []byte(`apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - deployment.yaml
  - service.yaml
  - httproute.yaml
patches:
  - path: patch-web.yaml
configMapGenerator:
  - name: kmc-env
    envs:
      - config.env
  - name: kmc-clusters
    files:
      - clusters.yaml
`),
		"k8s/kmc/base/patch-web.yaml": []byte("kind: Deployment\n"),
		"k8s/kmc/base/clusters.yaml":  []byte("clusters: []\n"),
		"k8s/kmc/overlays/prod/kustomization.yaml": []byte(`apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - ../../base
patches:
  - path: patch-cluster-tokens.yaml
configMapGenerator:
  - name: kmc-env
    behavior: replace
    envs:
      - config.env
`),
		"k8s/kmc/overlays/prod/patch-cluster-tokens.yaml": []byte("kind: Deployment\n"),
	}
	merged, err := MergeTrees(existing, generated)
	if err != nil {
		t.Fatal(err)
	}
	var base kustomization
	if err := yamlx.Unmarshal(merged["k8s/kmc/base/kustomization.yaml"], &base); err != nil {
		t.Fatal(err)
	}
	if !containsAll(base.Resources, "deployment.yaml", "service.yaml", "httproute.yaml") {
		t.Fatalf("base resources %+v", base.Resources)
	}
	if len(base.Patches) != 1 || base.Patches[0].Path != "patch-web.yaml" {
		t.Fatalf("base patches %+v", base.Patches)
	}
	if len(base.ConfigMapGenerator) != 2 {
		t.Fatalf("lost configMapGenerator: %+v", base.ConfigMapGenerator)
	}
	var overlay kustomization
	if err := yamlx.Unmarshal(merged[OverlayKustomizationPath(d, "prod")], &overlay); err != nil {
		t.Fatal(err)
	}
	if !containsAll(patchPaths(overlay.Patches), "patch-httproute.yaml", "patch-cluster-tokens.yaml") {
		t.Fatalf("prod patches %+v", overlay.Patches)
	}
	if strings.Contains(string(merged[OverlayKustomizationPath(d, "prod")]), "patch-volumes.yaml") {
		t.Fatalf("merge must not invent patch-volumes.yaml:\n%s", merged[OverlayKustomizationPath(d, "prod")])
	}
	for _, p := range []string{
		"k8s/kmc/base/patch-web.yaml",
		"k8s/kmc/base/clusters.yaml",
		"k8s/kmc/overlays/prod/patch-cluster-tokens.yaml",
	} {
		if string(merged[p]) != string(existing[p]) {
			t.Fatalf("lost extra file %s", p)
		}
	}
}

func patchPaths(patches []kustomizePatch) []string {
	out := make([]string, len(patches))
	for i, p := range patches {
		out[i] = p.Path
	}
	return out
}

func TestMergeKeepsConfigMapGenerator(t *testing.T) {
	t.Parallel()
	d := loadExample(t, "kmc")
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

func loadExample(t *testing.T, name string) *spec.Deployable {
	t.Helper()
	d, err := spec.Load(filepath.Join(repoRoot(t), "examples", name+".yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func containsAll(have []string, want ...string) bool {
	seen := map[string]struct{}{}
	for _, s := range have {
		seen[s] = struct{}{}
	}
	for _, s := range want {
		if _, ok := seen[s]; !ok {
			return false
		}
	}
	return true
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
