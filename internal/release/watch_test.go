package release

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ianunruh/deploybot/internal/argo"
	"github.com/ianunruh/deploybot/internal/catalog"
	"github.com/ianunruh/deploybot/internal/gitwrite"
	"github.com/ianunruh/deploybot/internal/render"
)

const miniKMC = `
apiVersion: deploybot.kcloud.io/v1alpha1
kind: Deployable
metadata:
  name: kmc
spec:
  namespace: kmc-system
  git:
    repoURL: https://github.com/ianunruh/kcloud-ops
    workloadPath: k8s/kmc
    applicationPath: k8s/apps/projects/sandbox
  argo:
    project: sandbox
  image:
    repository: ghcr.io/ianunruh/kmc
  workload:
    kind: Deployment
  stages:
    - name: homelab
    - name: prod
      promote:
        after:
          - bake
        bake: 1ns
`

func catalogYAML(t *testing.T, body string) *catalog.Catalog {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "kmc.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cat, err := catalog.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	return cat
}

func TestReconcileFlowsAutoPromote(t *testing.T) {
	t.Parallel()
	dir := initOpsRepo(t)
	fake := argo.NewFake()
	fake.Set("kmc", argo.Status{Health: "Healthy", Sync: "Synced"})
	svc := &Service{
		Catalog: catalogYAML(t, miniKMC),
		OpsRepo: dir,
		Apply:   true,
		Sync:    true,
		Argo:    argo.StaticRouter{Client: fake},
		Wait:    time.Second,
		Author:  gitwrite.Author{Name: "t", Email: "t@t"},
	}
	if _, err := svc.Pin(t.Context(), "kmc", "homelab", "ghcr.io/ianunruh/kmc@sha256:abc"); err != nil {
		t.Fatal(err)
	}
	svc.RefreshLive(t.Context())
	svc.ReconcileFlows(t.Context())
	tree := mustOpenTree(t, dir)
	d := mustKMC(t, svc)
	img, err := render.CurrentImage(tree, d, "prod")
	if err != nil {
		t.Fatal(err)
	}
	if img.Digest != "sha256:abc" {
		t.Fatalf("prod %+v", img)
	}
}

func TestReconcileFlowsApprovalDoesNotPromote(t *testing.T) {
	t.Parallel()
	body := `
apiVersion: deploybot.kcloud.io/v1alpha1
kind: Deployable
metadata:
  name: kmc
spec:
  namespace: kmc-system
  git:
    repoURL: https://github.com/ianunruh/kcloud-ops
    workloadPath: k8s/kmc
    applicationPath: k8s/apps/projects/sandbox
  argo:
    project: sandbox
  image:
    repository: ghcr.io/ianunruh/kmc
  workload:
    kind: Deployment
  stages:
    - name: homelab
    - name: prod
      promote:
        after:
          - approval
`
	dir := initOpsRepo(t)
	fake := argo.NewFake()
	fake.Set("kmc", argo.Status{Health: "Healthy", Sync: "Synced"})
	svc := &Service{
		Catalog: catalogYAML(t, body),
		OpsRepo: dir,
		Apply:   true,
		Sync:    true,
		Argo:    argo.StaticRouter{Client: fake},
		Wait:    time.Second,
		Author:  gitwrite.Author{Name: "t", Email: "t@t"},
	}
	if _, err := svc.Pin(t.Context(), "kmc", "homelab", "ghcr.io/ianunruh/kmc@sha256:abc"); err != nil {
		t.Fatal(err)
	}
	svc.RefreshLive(t.Context())
	svc.ReconcileFlows(t.Context())
	tree := mustOpenTree(t, dir)
	d := mustKMC(t, svc)
	if _, err := render.CurrentImage(tree, d, "prod"); err == nil {
		t.Fatal("approval must not auto-promote")
	}
}

func TestWatchFlowsExitsOnCancel(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	svc := &Service{Apply: true, Catalog: loadExamples(t), FlowEvery: time.Hour}
	done := make(chan struct{})
	go func() {
		svc.WatchFlows(ctx)
		close(done)
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("WatchFlows did not exit")
	}
}
