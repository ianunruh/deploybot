package image

import (
	"regexp"
	"strings"
	"time"
)

type digestGroup struct {
	digest    string
	tags      []string
	createdAt time.Time
	repo      string
}

// Matching keeps versions that have at least one tag matching re. Remaining
// tags are rewritten so PreferredTag can pick them. A nil regex is a no-op.
func Matching(versions []Version, re *regexp.Regexp) []Version {
	if re == nil {
		return versions
	}
	out := make([]Version, 0, len(versions))
	for _, v := range versions {
		var tags []string
		for _, t := range versionTags(v) {
			if t == "" || skipArchTag(t) || !re.MatchString(t) {
				continue
			}
			tags = append(tags, t)
		}
		if len(tags) == 0 {
			continue
		}
		v.Tags = tags
		v.Tag = PreferredTag(tags)
		ref := Ref{Repository: v.Repository, Tag: v.Tag, Digest: v.Digest}
		v.Ref = ref.String()
		out = append(out, v)
	}
	return out
}

// Newest picks the newest published digest, preferring a stable tag on that
// digest over latest/nightly. Hub listings are one row per tag; GHCR already
// groups tags on a digest.
func Newest(versions []Version) (Version, bool) {
	groups := map[string]*digestGroup{}
	order := []string{}
	for _, v := range versions {
		key := groupKey(v)
		if key == "" {
			continue
		}
		g, ok := groups[key]
		if !ok {
			g = &digestGroup{digest: strings.TrimSpace(v.Digest), repo: v.Repository}
			groups[key] = g
			order = append(order, key)
		}
		if g.repo == "" {
			g.repo = v.Repository
		}
		for _, t := range versionTags(v) {
			if t == "" || skipArchTag(t) {
				continue
			}
			g.tags = append(g.tags, t)
		}
		if !v.CreatedAt.IsZero() && (g.createdAt.IsZero() || v.CreatedAt.After(g.createdAt)) {
			g.createdAt = v.CreatedAt
		}
	}

	var stable, unstable *digestGroup
	for _, key := range order {
		g := groups[key]
		if len(g.tags) == 0 {
			continue
		}
		tag := PreferredTag(g.tags)
		if isUnstableTag(tag) {
			if unstable == nil || newerGroup(g, unstable) {
				unstable = g
			}
			continue
		}
		if stable == nil || newerGroup(g, stable) {
			stable = g
		}
	}
	winner := stable
	if winner == nil {
		winner = unstable
	}
	if winner == nil {
		return Version{}, false
	}
	return versionFromGroup(winner), true
}

func versionFromGroup(g *digestGroup) Version {
	tag := PreferredTag(g.tags)
	ref := Ref{Repository: g.repo, Tag: tag, Digest: g.digest}
	return Version{
		Repository: g.repo,
		Ref:        ref.String(),
		Tag:        tag,
		Digest:     g.digest,
		Tags:       append([]string{}, g.tags...),
		CreatedAt:  g.createdAt,
	}
}

func newerGroup(a, b *digestGroup) bool {
	if b == nil {
		return true
	}
	if a.createdAt.IsZero() {
		return false
	}
	if b.createdAt.IsZero() {
		return true
	}
	return a.createdAt.After(b.createdAt)
}

func groupKey(v Version) string {
	if d := strings.TrimSpace(v.Digest); d != "" {
		return d
	}
	tag := strings.TrimSpace(v.Tag)
	if tag == "" && len(v.Tags) > 0 {
		tag = strings.TrimSpace(v.Tags[0])
	}
	if tag == "" {
		return ""
	}
	return "tag:" + tag
}

func versionTags(v Version) []string {
	if len(v.Tags) > 0 {
		return v.Tags
	}
	if v.Tag != "" {
		return []string{v.Tag}
	}
	return nil
}
