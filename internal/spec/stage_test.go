package spec

import (
	"strings"
	"testing"
	"time"
)

func TestUnknownStage(t *testing.T) {
	t.Parallel()
	d, err := Parse([]byte(kmcYAML))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Stage("gamma"); err == nil {
		t.Fatal("expected error")
	}
}

func TestParsePromotePolicy(t *testing.T) {
	t.Parallel()
	body := kmcYAML + `
      promote:
        after:
          - bake
          - approval
        bake: 30m
`
	d, err := Parse([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	st, err := d.Stage("prod")
	if err != nil {
		t.Fatal(err)
	}
	if st.Promote == nil || !st.Promote.Has(AfterBake) || !st.Promote.Has(AfterApproval) {
		t.Fatalf("promote %+v", st.Promote)
	}
	if st.Promote.Bake.Duration() != 30*time.Minute {
		t.Fatalf("bake %s", st.Promote.Bake.Duration())
	}
	if st.Promote.AutoPromote() {
		t.Fatal("approval should block auto-promote")
	}
	src, err := d.SourceStage("prod")
	if err != nil {
		t.Fatal(err)
	}
	if src != "homelab" {
		t.Fatalf("source %q", src)
	}
}

func TestParsePromoteAfterMustBeList(t *testing.T) {
	t.Parallel()
	body := kmcYAML + `
      promote:
        after: approval
`
	if _, err := Parse([]byte(body)); err == nil {
		t.Fatal("expected scalar after to be rejected")
	}
}

func TestParsePromoteRejectsFirstStage(t *testing.T) {
	t.Parallel()
	body := strings.Replace(kmcYAML, "    - name: homelab\n", `    - name: homelab
      promote:
        after:
          - approval
`, 1)
	if _, err := Parse([]byte(body)); err == nil {
		t.Fatal("expected first-stage promote rejected")
	}
}

func TestParsePromoteBakeRequired(t *testing.T) {
	t.Parallel()
	body := kmcYAML + `
      promote:
        after:
          - bake
`
	if _, err := Parse([]byte(body)); err == nil {
		t.Fatal("expected bake duration required")
	}
}
