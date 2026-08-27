package release

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/ianunruh/deploybot/internal/gitwrite"
	"github.com/ianunruh/deploybot/internal/valkey"
)

func TestOverlayValkeyHydratesHistory(t *testing.T) {
	t.Parallel()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go (&valkey.Memory{}).Listen(ln)

	dir := initOpsRepo(t)
	cat := catalogYAML(t, miniKMC)
	addr := ln.Addr().String()
	svc := &Service{
		Catalog: cat,
		OpsRepo: dir,
		Apply:   true,
		Author:  gitwrite.Author{Name: "t", Email: "t@t"},
		Valkey:  addr,
	}
	if _, err := svc.Pin(t.Context(), "kmc", "homelab", "ghcr.io/ianunruh/kmc:main-dead@sha256:abc"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Promote(t.Context(), "kmc", "homelab", "prod", ""); err != nil {
		t.Fatal(err)
	}
	svc.RefreshOverlays(t.Context())

	cold := &Service{Catalog: cat, OpsRepo: dir, Valkey: addr}
	cold.initCaches()
	cold.overlays.hydrate(t.Context())
	cold.overlays.mu.Lock()
	ready, n := cold.overlays.revsReady, len(cold.overlays.revs)
	cold.overlays.mu.Unlock()
	if !ready || n == 0 {
		t.Fatalf("hydrate ready=%v revs=%d", ready, n)
	}

	h, err := cold.History(t.Context(), "kmc", 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(h.Events) != 2 {
		t.Fatalf("history %+v", h.Events)
	}
	if h.Events[0].Kind != EventPromote || h.Events[1].Kind != EventPin {
		t.Fatalf("events %+v", h.Events)
	}
	global, err := cold.ListHistory(t.Context(), 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(global.Events) != 2 {
		t.Fatalf("global %+v", global.Events)
	}
}

func TestRefreshOverlaysIncremental(t *testing.T) {
	t.Parallel()
	dir := initOpsRepo(t)
	svc := &Service{
		Catalog: catalogYAML(t, miniKMC),
		OpsRepo: dir,
		Apply:   true,
		Author:  gitwrite.Author{Name: "t", Email: "t@t"},
	}
	if _, err := svc.Pin(t.Context(), "kmc", "homelab", "ghcr.io/ianunruh/kmc:main-dead@sha256:abc"); err != nil {
		t.Fatal(err)
	}
	svc.RefreshOverlays(t.Context())
	if _, err := svc.Pin(t.Context(), "kmc", "homelab", "ghcr.io/ianunruh/kmc:main-next@sha256:def"); err != nil {
		t.Fatal(err)
	}
	svc.RefreshOverlays(t.Context())

	h, err := svc.History(t.Context(), "kmc", 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(h.Events) != 2 {
		t.Fatalf("events %+v", h.Events)
	}
	if h.Events[0].Digest != "sha256:def" || h.Events[1].Digest != "sha256:abc" {
		t.Fatalf("order %+v", h.Events)
	}
	global, err := svc.ListHistory(t.Context(), 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(global.Events) != 2 {
		t.Fatalf("global %+v", global.Events)
	}
}

func TestWatchOverlaysExitsOnCancel(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	svc := &Service{Catalog: loadExamples(t), OverlayEvery: time.Hour}
	done := make(chan struct{})
	go func() {
		svc.WatchOverlays(ctx)
		close(done)
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("WatchOverlays did not exit")
	}
}
