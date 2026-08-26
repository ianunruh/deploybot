package image

import (
	"regexp"
	"testing"
	"time"
)

func TestNewestEmpty(t *testing.T) {
	t.Parallel()
	if _, ok := Newest(nil); ok {
		t.Fatal("expected no version")
	}
	if _, ok := Newest([]Version{}); ok {
		t.Fatal("expected no version")
	}
}

func TestNewestHubTagsSameDigest(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	got, ok := Newest([]Version{
		hubVer("latest", "sha256:aaa", now.Add(time.Hour)),
		hubVer("4.0.16.2945-ls286", "sha256:aaa", now),
	})
	if !ok {
		t.Fatal("expected version")
	}
	if got.Tag != "4.0.16.2945-ls286" || got.Digest != "sha256:aaa" {
		t.Fatalf("%+v", got)
	}
	if got.Ref != "docker.io/linuxserver/sonarr:4.0.16.2945-ls286@sha256:aaa" {
		t.Fatalf("ref %q", got.Ref)
	}
}

func TestNewestNewerDigestWins(t *testing.T) {
	t.Parallel()
	old := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	newer := old.Add(24 * time.Hour)
	got, ok := Newest([]Version{
		hubVer("4.0.15.2941-ls285", "sha256:old", old),
		hubVer("4.0.16.2945-ls286", "sha256:new", newer),
		hubVer("latest", "sha256:new", newer),
	})
	if !ok {
		t.Fatal("expected version")
	}
	if got.Tag != "4.0.16.2945-ls286" || got.Digest != "sha256:new" {
		t.Fatalf("%+v", got)
	}
}

func TestNewestSkipsNightlyWhenStableExists(t *testing.T) {
	t.Parallel()
	old := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	newer := old.Add(time.Hour)
	got, ok := Newest([]Version{
		hubVer("4.0.15.2941-ls285", "sha256:stable", old),
		hubVer("nightly", "sha256:night", newer),
	})
	if !ok {
		t.Fatal("expected version")
	}
	if got.Tag != "4.0.15.2941-ls285" || got.Digest != "sha256:stable" {
		t.Fatalf("%+v", got)
	}
}

func TestNewestSkipsDevelopWhenStableExists(t *testing.T) {
	t.Parallel()
	old := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	newer := old.Add(time.Hour)
	got, ok := Newest([]Version{
		hubVer("v2.15.2-ls194", "sha256:stable", old),
		hubVer("develop-version-4fb29baa", "sha256:dev", newer),
		hubVer("1.6.1-development", "sha256:dev2", newer.Add(time.Minute)),
	})
	if !ok {
		t.Fatal("expected version")
	}
	if got.Tag != "v2.15.2-ls194" || got.Digest != "sha256:stable" {
		t.Fatalf("%+v", got)
	}
}

func TestNewestSkipsTestingWhenStableExists(t *testing.T) {
	t.Parallel()
	old := time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC)
	newer := old.Add(8 * time.Hour)
	got, ok := Newest([]Version{
		hubVer("26.2.20260821", "sha256:stable", old),
		hubVer("v26.2-ls257", "sha256:stable", old),
		hubVer("testing-version-c4f9313", "sha256:test", newer),
		hubVer("testing", "sha256:test", newer),
	})
	if !ok {
		t.Fatal("expected version")
	}
	if got.Tag != "26.2.20260821" || got.Digest != "sha256:stable" {
		t.Fatalf("%+v", got)
	}
}

func TestNewestFallsBackToLatest(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	got, ok := Newest([]Version{
		hubVer("latest", "sha256:aaa", now),
	})
	if !ok {
		t.Fatal("expected version")
	}
	if got.Tag != "latest" || got.Digest != "sha256:aaa" {
		t.Fatalf("%+v", got)
	}
}

func TestPreferredTagSkipsNightly(t *testing.T) {
	t.Parallel()
	if got := PreferredTag([]string{"nightly", "v1.2.3"}); got != "v1.2.3" {
		t.Fatalf("got %q", got)
	}
}

func TestMatchingFiltersLinuxServerAliases(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 25, 21, 51, 0, 0, time.UTC)
	re := regexp.MustCompile(`^v?\d+(\.\d+)+-ls\d+$`)
	got := Matching([]Version{
		hubVer("1.6.0", "sha256:new", now),
		hubVer("version-v1.6.0", "sha256:new", now),
		hubVer("v1.6.0-ls361", "sha256:new", now),
		hubVer("latest", "sha256:new", now),
		hubVer("1.6.1-development", "sha256:dev", now.Add(-time.Hour)),
		hubVer("amd64-v1.6.0-ls361", "sha256:arch", now),
	}, re)
	if len(got) != 1 || got[0].Tag != "v1.6.0-ls361" {
		t.Fatalf("%+v", got)
	}
	newest, ok := Newest(got)
	if !ok || newest.Tag != "v1.6.0-ls361" || newest.Digest != "sha256:new" {
		t.Fatalf("%+v ok=%v", newest, ok)
	}
}

func TestMatchingNilIsNoop(t *testing.T) {
	t.Parallel()
	in := []Version{hubVer("1.6.0", "sha256:new", time.Now())}
	got := Matching(in, nil)
	if len(got) != 1 || got[0].Tag != "1.6.0" {
		t.Fatalf("%+v", got)
	}
}

func hubVer(tag, digest string, at time.Time) Version {
	ref := Ref{Repository: "docker.io/linuxserver/sonarr", Tag: tag, Digest: digest}
	return Version{
		Repository: ref.Repository,
		Ref:        ref.String(),
		Tag:        tag,
		Digest:     digest,
		Tags:       []string{tag},
		CreatedAt:  at,
	}
}
