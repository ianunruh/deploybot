package render

import (
	"strings"
	"testing"

	"github.com/ianunruh/deploybot/internal/image"
	"github.com/ianunruh/deploybot/internal/yamlx"
)

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

func patchPaths(patches []kustomizePatch) []string {
	out := make([]string, len(patches))
	for i, p := range patches {
		out[i] = p.Path
	}
	return out
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
