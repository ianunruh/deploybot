package release

import (
	"context"
	"fmt"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ianunruh/deploybot/internal/argo"
	"github.com/ianunruh/deploybot/internal/gitwrite"
	"github.com/ianunruh/deploybot/internal/render"
	"github.com/ianunruh/deploybot/internal/valkey"
)

func TestRefreshLiveFeedsStatus(t *testing.T) {
	t.Parallel()
	homelab := argo.NewFake()
	homelab.Set("kmc", argo.Status{Health: "Healthy", Sync: "Synced"})
	prod := argo.NewFake()
	prod.Set("kmc", argo.Status{Health: "Progressing", Sync: "OutOfSync"})
	svc := &Service{
		Catalog: loadExamples(t),
		Argo:    stageRouter{"homelab": homelab, "prod": prod},
	}
	svc.RefreshLive(t.Context())
	_, hList := homelab.Calls()
	_, pList := prod.Calls()
	if hList != 1 || pList != 1 {
		t.Fatalf("refresh lists homelab %d prod %d", hList, pList)
	}

	st, err := svc.Status(t.Context(), "kmc")
	if err != nil {
		t.Fatal(err)
	}
	if st.Stages[0].Health != "Healthy" || st.Stages[1].Health != "Progressing" {
		t.Fatalf("stages %+v", st.Stages)
	}
	if st.Stages[0].Connected == nil || !*st.Stages[0].Connected {
		t.Fatalf("homelab connected %+v", st.Stages[0].Connected)
	}
	if st.Stages[0].UpdatedAt == nil {
		t.Fatal("expected updatedAt")
	}
	_, hList = homelab.Calls()
	_, pList = prod.Calls()
	if hList != 1 || pList != 1 {
		t.Fatalf("status should not list: homelab %d prod %d", hList, pList)
	}

	if _, err := svc.LiveStatus(t.Context(), "kmc"); err != nil {
		t.Fatal(err)
	}
	_, hList = homelab.Calls()
	_, pList = prod.Calls()
	if hList != 1 || pList != 1 {
		t.Fatalf("live status should not list: homelab %d prod %d", hList, pList)
	}
}

func TestWatchingSkipsPullThrough(t *testing.T) {
	t.Parallel()
	homelab := argo.NewFake()
	homelab.Set("kmc", argo.Status{Health: "Healthy", Sync: "Synced"})
	svc := &Service{
		Catalog: loadExamples(t),
		Argo:    stageRouter{"homelab": homelab},
	}
	svc.SetLiveWatching()
	st, err := svc.Status(t.Context(), "kmc")
	if err != nil {
		t.Fatal(err)
	}
	if st.Stages[0].Health != "unknown" {
		t.Fatalf("health %q", st.Stages[0].Health)
	}
	if st.Stages[0].Message != "waiting for live snapshot" {
		t.Fatalf("message %q", st.Stages[0].Message)
	}
	_, hList := homelab.Calls()
	if hList != 0 {
		t.Fatalf("watching listed %d", hList)
	}
}

func TestDisconnectedKeepsLastHealth(t *testing.T) {
	t.Parallel()
	inner := argo.NewFake()
	inner.Set("kmc", argo.Status{Health: "Healthy", Sync: "Synced"})
	flaky := &flakyList{inner: inner}
	svc := &Service{
		Catalog: catalogYAML(t, miniKMC),
		Argo:    stageRouter{"homelab": flaky, "prod": flaky},
	}
	svc.RefreshLive(t.Context())
	flaky.fail.Store(true)
	svc.RefreshLive(t.Context())

	st, err := svc.Status(t.Context(), "kmc")
	if err != nil {
		t.Fatal(err)
	}
	if st.Stages[0].Health != "Healthy" {
		t.Fatalf("kept health %+v", st.Stages[0])
	}
	if st.Stages[0].Connected == nil || *st.Stages[0].Connected {
		t.Fatalf("expected disconnected %+v", st.Stages[0].Connected)
	}
	if st.Stages[0].Message == "" {
		t.Fatal("expected disconnect message")
	}
}

func TestWatchLiveExitsOnCancel(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	svc := &Service{Catalog: loadExamples(t), LiveEvery: time.Hour}
	done := make(chan struct{})
	go func() {
		svc.WatchLive(ctx)
		close(done)
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("WatchLive did not exit")
	}
}

func TestLiveStoreValkeyRoundTrip(t *testing.T) {
	t.Parallel()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go (&valkey.Memory{}).Listen(ln)

	store := newLiveStore(ln.Addr().String())
	at := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	store.put("homelab", clusterSnap{
		UpdatedAt: at,
		CheckedAt: at,
		Connected: true,
		Apps:      map[string]argo.Status{"kmc": {Name: "kmc", Health: "Healthy", Sync: "Synced"}},
	})

	cold := newLiveStore(ln.Addr().String())
	cold.hydrate(t.Context(), []string{"homelab"})
	got, ok := cold.get("homelab")
	if !ok || !got.Connected || got.Apps["kmc"].Health != "Healthy" {
		t.Fatalf("hydrate %+v ok=%v", got, ok)
	}
	if !got.UpdatedAt.Equal(at) {
		t.Fatalf("updatedAt %s", got.UpdatedAt)
	}
}

func TestReconcileFlowsStaleDoesNotPromote(t *testing.T) {
	t.Parallel()
	dir := initOpsRepo(t)
	inner := argo.NewFake()
	inner.Set("kmc", argo.Status{Health: "Healthy", Sync: "Synced"})
	flaky := &flakyList{inner: inner}
	svc := &Service{
		Catalog: catalogYAML(t, miniKMC),
		OpsRepo: dir,
		Apply:   true,
		Sync:    true,
		Argo:    argo.StaticRouter{Client: flaky},
		Wait:    time.Second,
		Author:  gitwrite.Author{Name: "t", Email: "t@t"},
	}
	if _, err := svc.Pin(t.Context(), "kmc", "homelab", "ghcr.io/ianunruh/kmc@sha256:abc"); err != nil {
		t.Fatal(err)
	}
	svc.RefreshLive(t.Context())
	flaky.fail.Store(true)
	svc.RefreshLive(t.Context())
	svc.ReconcileFlows(t.Context())

	tree := mustOpenTree(t, dir)
	d := mustKMC(t, svc)
	if _, err := render.CurrentImage(tree, d, "prod"); err == nil {
		t.Fatal("stale healthy must not auto-promote")
	}
}

type flakyList struct {
	inner *argo.Fake
	fail  atomic.Bool
}

func (f *flakyList) Get(ctx context.Context, app string) (argo.Status, error) {
	return f.inner.Get(ctx, app)
}

func (f *flakyList) List(ctx context.Context) ([]argo.Status, error) {
	if f.fail.Load() {
		return nil, fmt.Errorf("dial tcp: i/o timeout")
	}
	return f.inner.List(ctx)
}

func (f *flakyList) Sync(ctx context.Context, app string, prune bool) error {
	return f.inner.Sync(ctx, app, prune)
}
