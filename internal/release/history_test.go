package release

import (
	"testing"
	"time"

	"github.com/ianunruh/deploybot/internal/argo"
	"github.com/ianunruh/deploybot/internal/gitwrite"
)

func TestHistoryPinThenPromote(t *testing.T) {
	t.Parallel()
	dir := initOpsRepo(t)
	fake := argo.NewFake()
	fake.Set("kmc", argo.Status{Health: "Healthy", Sync: "Synced"})
	svc := &Service{
		Catalog: loadExamples(t),
		OpsRepo: dir,
		Apply:   true,
		Sync:    true,
		Argo:    argo.StaticRouter{Client: fake},
		Wait:    time.Second,
		Author:  gitwrite.Author{Name: "t", Email: "t@t"},
	}
	if _, err := svc.Pin(t.Context(), "kmc", "homelab", "ghcr.io/ianunruh/kmc:main-dead@sha256:abc"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Promote(t.Context(), "kmc", "homelab", "prod", ""); err != nil {
		t.Fatal(err)
	}
	h, err := svc.History(t.Context(), "kmc", 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(h.Events) != 2 {
		t.Fatalf("events %+v", h.Events)
	}
	if h.Events[0].Kind != EventPromote || h.Events[0].Stage != "prod" {
		t.Fatalf("newest %+v", h.Events[0])
	}
	if h.Events[1].Kind != EventPin || h.Events[1].Stage != "homelab" {
		t.Fatalf("older %+v", h.Events[1])
	}
	if len(h.Releases) != 1 || !h.Releases[0].Current {
		t.Fatalf("releases %+v", h.Releases)
	}
	rel := h.Releases[0]
	if rel.Digest != "sha256:abc" {
		t.Fatalf("digest %q", rel.Digest)
	}
	if rel.Stages["homelab"].Kind != EventPin || rel.Stages["prod"].Kind != EventPromote {
		t.Fatalf("stages %+v", rel.Stages)
	}
	st, err := svc.Status(t.Context(), "kmc")
	if err != nil {
		t.Fatal(err)
	}
	if st.Flow.Hops[0].State != HopCaughtUp {
		t.Fatalf("flow %+v", st.Flow)
	}
	if st.Stages[0].PinnedAt == nil || st.Stages[1].PinnedAt == nil {
		t.Fatalf("pinnedAt %+v", st.Stages)
	}
}

func TestHistoryUnknownDeployable(t *testing.T) {
	t.Parallel()
	svc := &Service{Catalog: loadExamples(t)}
	if _, err := svc.History(t.Context(), "nope", 0); err == nil {
		t.Fatal("expected error")
	}
}
