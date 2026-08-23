package diffx

import (
	"fmt"
	"strings"

	"github.com/ianunruh/deploybot/internal/render"
)

func Trees(before, after render.Tree) string {
	var b strings.Builder
	seen := map[string]struct{}{}
	for _, p := range render.SortedPaths(after) {
		seen[p] = struct{}{}
		old := before[p]
		neu := after[p]
		if string(old) == string(neu) {
			continue
		}
		fmt.Fprintf(&b, "--- a/%s\n+++ b/%s\n", p, p)
		if len(old) == 0 {
			fmt.Fprintf(&b, "%s", indentPlus(string(neu)))
			continue
		}
		fileDiff(&b, string(old), string(neu))
	}
	for _, p := range render.SortedPaths(before) {
		if _, ok := seen[p]; ok {
			continue
		}
		fmt.Fprintf(&b, "--- a/%s\n+++ /dev/null\n%s", p, indentMinus(string(before[p])))
	}
	return b.String()
}

func fileDiff(b *strings.Builder, old, neu string) {
	// Line-oriented, no LCS: emit removed old then added new. Good enough
	// to preview a pin in the UI without another dependency.
	if old != "" {
		fmt.Fprintf(b, "-%s", indentMinus(old))
	}
	if neu != "" {
		fmt.Fprintf(b, "+%s", indentPlus(neu))
	}
}

func indentPlus(s string) string {
	return prefixLines(s, "+")
}

func indentMinus(s string) string {
	return prefixLines(s, "-")
}

func prefixLines(s, prefix string) string {
	if s == "" {
		return ""
	}
	lines := strings.Split(strings.TrimSuffix(s, "\n"), "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n") + "\n"
}
