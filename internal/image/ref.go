package image

import (
	"fmt"
	"strings"
	"unicode"
)

// Ref is a container image name with an optional tag and/or digest.
type Ref struct {
	Repository string
	Tag        string
	Digest     string
}

func Parse(s string) (Ref, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Ref{}, fmt.Errorf("empty image reference")
	}
	repoTag, digest, ok := strings.Cut(s, "@")
	if ok && digest == "" {
		return Ref{}, fmt.Errorf("invalid image %q: empty digest", s)
	}
	if !ok {
		digest = ""
	}
	repository, tag := splitRepoTag(repoTag)
	if repository == "" {
		return Ref{}, fmt.Errorf("invalid image %q", s)
	}
	return Ref{Repository: repository, Tag: tag, Digest: digest}, nil
}

func MustParse(s string) Ref {
	r, err := Parse(s)
	if err != nil {
		panic(err)
	}
	return r
}

func (r Ref) String() string {
	s := r.Repository
	if r.Tag != "" {
		s += ":" + r.Tag
	}
	if r.Digest != "" {
		s += "@" + r.Digest
	}
	return s
}

// LogName is the commit-subject form: repo:tag, omitting the full digest.
func (r Ref) LogName() string {
	s := r.Repository
	if r.Tag != "" {
		return s + ":" + r.Tag
	}
	if r.Digest != "" {
		return s + "@" + r.ShortDigest()
	}
	return s
}

// Compact is the UI form: tag and a truncated digest, without the repository.
func (r Ref) Compact() string {
	s := r.Tag
	if s == "" {
		s = r.Repository
	}
	if r.Digest == "" {
		return s
	}
	if s == "" {
		return r.ShortDigest()
	}
	return s + "@" + r.ShortDigest()
}

func (r Ref) ShortDigest() string {
	d := r.Digest
	if d == "" {
		return ""
	}
	hex, hadSHA := strings.CutPrefix(d, "sha256:")
	if len(hex) > 12 {
		hex = hex[:12]
	}
	if hadSHA {
		return "sha256:" + hex
	}
	return hex
}

func (r Ref) IsZero() bool {
	return r.Repository == "" && r.Tag == "" && r.Digest == ""
}

// ReleaseKey groups pins of the same image. Digest wins; otherwise the full ref.
func (r Ref) ReleaseKey() string {
	if r.Digest != "" {
		return r.Digest
	}
	return r.String()
}

// SameRelease is true when both refs are the same digest, or the same full ref
// when a digest is missing.
func (r Ref) SameRelease(o Ref) bool {
	if r.IsZero() || o.IsZero() {
		return r.IsZero() && o.IsZero()
	}
	if r.Digest != "" && o.Digest != "" {
		return r.Digest == o.Digest
	}
	return r.String() == o.String()
}

func splitRepoTag(s string) (string, string) {
	i := strings.LastIndex(s, ":")
	if i < 0 {
		return s, ""
	}
	if slash := strings.LastIndex(s, "/"); slash > i {
		return s, ""
	}
	if !strings.Contains(s, "/") {
		rest := s[i+1:]
		if rest != "" && isAllDigits(rest) {
			return s, ""
		}
	}
	return s[:i], s[i+1:]
}

func isAllDigits(s string) bool {
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}
