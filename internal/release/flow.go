package release

import (
	"fmt"
	"strings"
	"time"

	"github.com/ianunruh/deploybot/internal/image"
	"github.com/ianunruh/deploybot/internal/spec"
)

const (
	HopCaughtUp        = "caught_up"
	HopDestAhead       = "dest_ahead"
	HopSourceUnhealthy = "source_unhealthy"
	HopBaking          = "baking"
	HopWaitingApproval = "waiting_approval"
	HopReady           = "ready"
	HopBehind          = "behind"
)

type Flow struct {
	Image  string `json:"image,omitempty"`
	Digest string `json:"digest,omitempty"`
	Tag    string `json:"tag,omitempty"`
	Hops   []Hop  `json:"hops"`
}

type Hop struct {
	From        string     `json:"from"`
	To          string     `json:"to"`
	State       string     `json:"state"`
	Gate        string     `json:"gate,omitempty"`
	Remaining   string     `json:"remaining,omitempty"`
	BakeUntil   *time.Time `json:"bakeUntil,omitempty"`
	SourceImage string     `json:"sourceImage,omitempty"`
	DestImage   string     `json:"destImage,omitempty"`
}

type stageSnap struct {
	name     string
	ref      image.Ref
	health   string
	pinnedAt *time.Time
	policy   *spec.PromotePolicy
	hasArgo  bool
}

func buildFlow(snaps []stageSnap, now time.Time) Flow {
	out := Flow{Hops: []Hop{}}
	if len(snaps) > 0 && !snaps[0].ref.IsZero() {
		out.Image = snaps[0].ref.Compact()
		out.Digest = snaps[0].ref.Digest
		out.Tag = snaps[0].ref.Tag
	}
	for i := 1; i < len(snaps); i++ {
		out.Hops = append(out.Hops, hopBetween(snaps[i-1], snaps[i], now))
	}
	return out
}

func hopBetween(src, dest stageSnap, now time.Time) Hop {
	h := Hop{From: src.name, To: dest.name}
	if !src.ref.IsZero() {
		h.SourceImage = src.ref.String()
	}
	if !dest.ref.IsZero() {
		h.DestImage = dest.ref.String()
	}
	if src.ref.SameRelease(dest.ref) {
		h.State = HopCaughtUp
		return h
	}
	if dest.pinnedAt != nil && src.pinnedAt != nil && dest.pinnedAt.After(*src.pinnedAt) && !dest.ref.IsZero() {
		h.State = HopDestAhead
		return h
	}
	p := dest.policy
	if p != nil && p.Has(spec.AfterBake) {
		if src.pinnedAt != nil {
			until := src.pinnedAt.Add(p.Bake.Duration()).UTC()
			h.BakeUntil = &until
			if now.Before(until) {
				h.State = HopBaking
				h.Gate = spec.AfterBake
				h.Remaining = formatRemaining(until.Sub(now))
				return h
			}
		} else {
			h.State = HopBaking
			h.Gate = spec.AfterBake
			return h
		}
	}
	if src.hasArgo && !strings.EqualFold(src.health, "Healthy") {
		h.State = HopSourceUnhealthy
		return h
	}
	if p != nil && p.Has(spec.AfterApproval) {
		h.State = HopWaitingApproval
		h.Gate = spec.AfterApproval
		return h
	}
	if p != nil && p.AutoPromote() {
		h.State = HopReady
		return h
	}
	h.State = HopBehind
	return h
}

func formatRemaining(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	d = d.Round(time.Second)
	h := d / time.Hour
	m := (d % time.Hour) / time.Minute
	s := (d % time.Minute) / time.Second
	switch {
	case h > 0 && m > 0:
		return fmt.Sprintf("%dh%dm", h, m)
	case h > 0:
		return fmt.Sprintf("%dh", h)
	case m > 0 && s > 0 && m < 5:
		return fmt.Sprintf("%dm%ds", m, s)
	case m > 0:
		return fmt.Sprintf("%dm", m)
	default:
		return fmt.Sprintf("%ds", s)
	}
}
