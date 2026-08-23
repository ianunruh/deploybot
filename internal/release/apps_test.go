package release

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ianunruh/deploybot/internal/argo"
)

func TestAppsCacheTTLAndSingleflight(t *testing.T) {
	t.Parallel()
	var lists atomic.Int32
	started := make(chan struct{})
	release := make(chan struct{})
	list := func(ctx context.Context) (map[string]argo.Status, error) {
		n := lists.Add(1)
		if n == 1 {
			close(started)
			select {
			case <-release:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		return map[string]argo.Status{
			"kmc": {Name: "kmc", Health: "Healthy", Sync: "Synced"},
		}, nil
	}
	c := newAppsCache(50 * time.Millisecond)

	var wg sync.WaitGroup
	errCh := make(chan error, 2)
	for range 2 {
		wg.Go(func() {
			st, err := c.lookup(t.Context(), "homelab", "kmc", list)
			if err != nil {
				errCh <- err
				return
			}
			if st.Health != "Healthy" {
				errCh <- fmt.Errorf("health %q", st.Health)
			}
		})
	}
	<-started
	time.Sleep(20 * time.Millisecond)
	if got := lists.Load(); got != 1 {
		t.Fatalf("inflight lists %d", got)
	}
	close(release)
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatal(err)
		}
	}

	st, err := c.lookup(t.Context(), "homelab", "kmc", list)
	if err != nil || st.Health != "Healthy" {
		t.Fatalf("%+v %v", st, err)
	}
	if got := lists.Load(); got != 1 {
		t.Fatalf("ttl hit lists %d", got)
	}

	time.Sleep(60 * time.Millisecond)
	if _, err := c.lookup(t.Context(), "homelab", "kmc", list); err != nil {
		t.Fatal(err)
	}
	if got := lists.Load(); got != 2 {
		t.Fatalf("after ttl lists %d", got)
	}

	c.drop("homelab")
	if _, err := c.lookup(t.Context(), "homelab", "kmc", list); err != nil {
		t.Fatal(err)
	}
	if got := lists.Load(); got != 3 {
		t.Fatalf("after drop lists %d", got)
	}
}

func TestAppsCacheMissingApp(t *testing.T) {
	t.Parallel()
	c := newAppsCache(time.Minute)
	_, err := c.lookup(t.Context(), "homelab", "nope", func(context.Context) (map[string]argo.Status, error) {
		return map[string]argo.Status{"kmc": {Name: "kmc"}}, nil
	})
	if err == nil {
		t.Fatal("expected missing app")
	}
}

func TestAppsCacheDropsInflight(t *testing.T) {
	t.Parallel()
	started := make(chan struct{})
	release := make(chan struct{})
	var lists atomic.Int32
	list := func(ctx context.Context) (map[string]argo.Status, error) {
		lists.Add(1)
		close(started)
		select {
		case <-release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		return map[string]argo.Status{"kmc": {Name: "kmc", Health: "Progressing"}}, nil
	}
	c := newAppsCache(time.Minute)
	errCh := make(chan error, 1)
	go func() {
		_, err := c.lookup(t.Context(), "homelab", "kmc", list)
		errCh <- err
	}()
	<-started
	c.drop("homelab")
	close(release)
	if err := <-errCh; err != nil {
		t.Fatal(err)
	}
	_, err := c.lookup(t.Context(), "homelab", "kmc", func(context.Context) (map[string]argo.Status, error) {
		lists.Add(1)
		return map[string]argo.Status{"kmc": {Name: "kmc", Health: "Healthy"}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := lists.Load(); got != 2 {
		t.Fatalf("lists %d, inflight result should not have been cached after drop", got)
	}
}
