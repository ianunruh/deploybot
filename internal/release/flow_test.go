package release

import (
	"testing"
	"time"

	"github.com/ianunruh/deploybot/internal/image"
	"github.com/ianunruh/deploybot/internal/spec"
)

func TestHopCaughtUp(t *testing.T) {
	t.Parallel()
	ref := image.MustParse("ghcr.io/ianunruh/kmc@sha256:abc")
	hop := hopBetween(
		stageSnap{name: "homelab", ref: ref},
		stageSnap{name: "prod", ref: ref},
		time.Now(),
	)
	if hop.State != HopCaughtUp {
		t.Fatalf("state %q", hop.State)
	}
	empty := hopBetween(stageSnap{name: "homelab"}, stageSnap{name: "prod"}, time.Now())
	if empty.State != HopCaughtUp {
		t.Fatalf("unpinned %q", empty.State)
	}
}

func TestHopBakingThenApproval(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)
	pinned := now.Add(-10 * time.Minute)
	src := stageSnap{
		name:     "homelab",
		ref:      image.MustParse("ghcr.io/ianunruh/kmc@sha256:abc"),
		health:   "Healthy",
		pinnedAt: &pinned,
		hasArgo:  true,
	}
	dest := stageSnap{
		name: "prod",
		ref:  image.MustParse("ghcr.io/ianunruh/kmc@sha256:old"),
		policy: &spec.PromotePolicy{
			After: []string{spec.AfterBake, spec.AfterApproval},
			Bake:  spec.Duration(30 * time.Minute),
		},
	}
	hop := hopBetween(src, dest, now)
	if hop.State != HopBaking || hop.Remaining != "20m" {
		t.Fatalf("%+v", hop)
	}
	hop = hopBetween(src, dest, now.Add(30*time.Minute))
	if hop.State != HopWaitingApproval {
		t.Fatalf("after bake %+v", hop)
	}
}

func TestHopSourceStaleBlocksAutoPromote(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	src := stageSnap{
		name:         "homelab",
		ref:          image.MustParse("ghcr.io/ianunruh/kmc@sha256:abc"),
		health:       "Healthy",
		hasArgo:      true,
		disconnected: true,
	}
	dest := stageSnap{
		name: "prod",
		policy: &spec.PromotePolicy{
			After: []string{spec.AfterHealthy},
		},
	}
	hop := hopBetween(src, dest, now)
	if hop.State != HopSourceStale {
		t.Fatalf("%+v", hop)
	}
	dest.policy = &spec.PromotePolicy{After: []string{spec.AfterApproval}}
	hop = hopBetween(src, dest, now)
	if hop.State != HopWaitingApproval {
		t.Fatalf("approval still human %+v", hop)
	}
}

func TestHopReadyAutoPromote(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	src := stageSnap{
		name:    "homelab",
		ref:     image.MustParse("ghcr.io/ianunruh/kmc@sha256:abc"),
		health:  "Healthy",
		hasArgo: true,
	}
	dest := stageSnap{
		name: "prod",
		policy: &spec.PromotePolicy{
			After: []string{spec.AfterHealthy},
		},
	}
	hop := hopBetween(src, dest, now)
	if hop.State != HopReady {
		t.Fatalf("%+v", hop)
	}
}

func TestHopDestAhead(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	older := now.Add(-time.Hour)
	newer := now.Add(-time.Minute)
	hop := hopBetween(
		stageSnap{
			name:     "homelab",
			ref:      image.MustParse("ghcr.io/ianunruh/kmc@sha256:abc"),
			pinnedAt: &older,
		},
		stageSnap{
			name:     "prod",
			ref:      image.MustParse("ghcr.io/ianunruh/kmc@sha256:fff"),
			pinnedAt: &newer,
		},
		now,
	)
	if hop.State != HopDestAhead {
		t.Fatalf("%+v", hop)
	}
}

func TestFormatRemaining(t *testing.T) {
	t.Parallel()
	if got := formatRemaining(90 * time.Second); got != "1m30s" {
		t.Fatalf("%q", got)
	}
	if got := formatRemaining(2 * time.Hour); got != "2h" {
		t.Fatalf("%q", got)
	}
}
