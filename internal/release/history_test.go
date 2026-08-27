package release

import (
	"testing"
	"time"

	"github.com/ianunruh/deploybot/internal/argo"
	"github.com/ianunruh/deploybot/internal/gitwrite"
)

func TestEventKindIgnoresTrailers(t *testing.T) {
	t.Parallel()
	msg := "pin kmc homelab to ghcr.io/ianunruh/kmc@sha256:abc\n\nDeploybot-Actor: auto-pin"
	if got := eventKind(msg); got != EventPin {
		t.Fatalf("%q", got)
	}
	if got := eventKind("promote kmc homelab -> prod (img)\n\nDeploybot-Actor: user\nDeploybot-Actor-ID: ianunruh"); got != EventPromote {
		t.Fatalf("%q", got)
	}
	if got := eventKind("rollback kmc homelab to img\n\nDeploybot-Actor: github-actions"); got != EventRollback {
		t.Fatalf("%q", got)
	}
}

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
	if h.Events[0].Deployable != "kmc" || h.Events[0].Namespace != "kmc-system" || h.Events[0].Project != "sandbox" {
		t.Fatalf("event meta %+v", h.Events[0])
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

func TestHistoryRollbackEvent(t *testing.T) {
	t.Parallel()
	dir := initOpsRepo(t)
	svc := &Service{
		Catalog: loadExamples(t),
		OpsRepo: dir,
		Apply:   true,
		Author:  gitwrite.Author{Name: "t", Email: "t@t"},
	}
	if _, err := svc.Pin(t.Context(), "kmc", "homelab", "ghcr.io/ianunruh/kmc:main-dead@sha256:abc"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Pin(t.Context(), "kmc", "homelab", "ghcr.io/ianunruh/kmc:main-bad@sha256:def"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Rollback(t.Context(), "kmc", "homelab", "ghcr.io/ianunruh/kmc:main-dead@sha256:abc"); err != nil {
		t.Fatal(err)
	}
	h, err := svc.History(t.Context(), "kmc", 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(h.Events) < 3 || h.Events[0].Kind != EventRollback || h.Events[0].Stage != "homelab" {
		t.Fatalf("events %+v", h.Events)
	}
	if len(h.Releases) != 2 {
		t.Fatalf("releases %+v", h.Releases)
	}
	foundCurrent := false
	for _, rel := range h.Releases {
		if rel.Digest == "sha256:abc" && rel.Current {
			foundCurrent = true
		}
	}
	if !foundCurrent {
		t.Fatalf("current release %+v", h.Releases)
	}
}

func TestHistoryUnknownDeployable(t *testing.T) {
	t.Parallel()
	svc := &Service{Catalog: loadExamples(t)}
	if _, err := svc.History(t.Context(), "nope", 0); err == nil {
		t.Fatal("expected error")
	}
}

func TestListHistoryAcrossDeployables(t *testing.T) {
	t.Parallel()
	dir := initOpsRepo(t)
	svc := &Service{
		Catalog: loadExamples(t),
		OpsRepo: dir,
		Apply:   true,
		Author:  gitwrite.Author{Name: "t", Email: "t@t"},
	}
	if _, err := svc.Pin(t.Context(), "kmc", "homelab", "ghcr.io/ianunruh/kmc:main-dead@sha256:abc"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Promote(t.Context(), "kmc", "homelab", "prod", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Pin(t.Context(), "humpty", "homelab", "ghcr.io/ianunruh/humpty:main-dead@sha256:fff"); err != nil {
		t.Fatal(err)
	}
	h, err := svc.ListHistory(t.Context(), 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(h.Events) != 3 {
		t.Fatalf("events %+v", h.Events)
	}
	if h.Events[0].Kind != EventPin || h.Events[0].Deployable != "humpty" || h.Events[0].Stage != "homelab" {
		t.Fatalf("newest %+v", h.Events[0])
	}
	if h.Events[0].Namespace != "deploybot-system" || h.Events[0].Project != "sandbox" {
		t.Fatalf("humpty meta %+v", h.Events[0])
	}
	if h.Events[1].Kind != EventPromote || h.Events[1].Deployable != "kmc" || h.Events[1].Stage != "prod" {
		t.Fatalf("middle %+v", h.Events[1])
	}
	if h.Events[2].Kind != EventPin || h.Events[2].Deployable != "kmc" || h.Events[2].Stage != "homelab" {
		t.Fatalf("oldest %+v", h.Events[2])
	}
	clipped, err := svc.ListHistory(t.Context(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(clipped.Events) != 1 || clipped.Events[0].Deployable != "humpty" {
		t.Fatalf("limit %+v", clipped.Events)
	}
}

func TestListHistoryEmpty(t *testing.T) {
	t.Parallel()
	h, err := (&Service{Catalog: loadExamples(t)}).ListHistory(t.Context(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if h.Events == nil || len(h.Events) != 0 {
		t.Fatalf("%+v", h)
	}
}
