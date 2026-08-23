package diffx

import (
	"fmt"
	"strings"

	"github.com/ianunruh/deploybot/internal/render"
)

const contextLines = 3

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
		fileDiff(&b, string(old), string(neu))
	}
	for _, p := range render.SortedPaths(before) {
		if _, ok := seen[p]; ok {
			continue
		}
		fmt.Fprintf(&b, "--- a/%s\n+++ /dev/null\n", p)
		fileDiff(&b, string(before[p]), "")
	}
	return b.String()
}

type opKind int

const (
	opEq opKind = iota
	opDel
	opIns
)

type edit struct {
	kind opKind
	line string
}

func fileDiff(b *strings.Builder, old, neu string) {
	writeUnified(b, splitLines(old), splitLines(neu))
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(s, "\n"), "\n")
}

func writeUnified(b *strings.Builder, a, n []string) {
	eds := edits(a, n)
	if len(eds) == 0 {
		return
	}
	marked := make([]bool, len(eds))
	changed := false
	for i, e := range eds {
		if e.kind == opEq {
			continue
		}
		changed = true
		lo := max(0, i-contextLines)
		hi := min(len(eds)-1, i+contextLines)
		for j := lo; j <= hi; j++ {
			marked[j] = true
		}
	}
	if !changed {
		return
	}

	oldLine, newLine := 1, 1
	i := 0
	for i < len(eds) {
		if !marked[i] {
			oldLine++
			newLine++
			i++
			continue
		}
		j := i
		for j < len(eds) && marked[j] {
			j++
		}
		hunk := eds[i:j]
		oldStart, newStart := oldLine, newLine
		oldCount, newCount := 0, 0
		for _, e := range hunk {
			switch e.kind {
			case opEq:
				oldCount++
				newCount++
			case opDel:
				oldCount++
			case opIns:
				newCount++
			}
		}
		if oldCount == 0 {
			oldStart = oldLine - 1
		}
		if newCount == 0 {
			newStart = newLine - 1
		}
		fmt.Fprintf(b, "@@ -%d,%d +%d,%d @@\n", oldStart, oldCount, newStart, newCount)
		for _, e := range hunk {
			switch e.kind {
			case opEq:
				fmt.Fprintf(b, " %s\n", e.line)
				oldLine++
				newLine++
			case opDel:
				fmt.Fprintf(b, "-%s\n", e.line)
				oldLine++
			case opIns:
				fmt.Fprintf(b, "+%s\n", e.line)
				newLine++
			}
		}
		i = j
	}
}

func edits(a, n []string) []edit {
	dp := lcsTable(a, n)
	var out []edit
	i, j := 0, 0
	for i < len(a) && j < len(n) {
		if a[i] == n[j] {
			out = append(out, edit{opEq, a[i]})
			i++
			j++
			continue
		}
		if dp[i+1][j] >= dp[i][j+1] {
			out = append(out, edit{opDel, a[i]})
			i++
			continue
		}
		out = append(out, edit{opIns, n[j]})
		j++
	}
	for ; i < len(a); i++ {
		out = append(out, edit{opDel, a[i]})
	}
	for ; j < len(n); j++ {
		out = append(out, edit{opIns, n[j]})
	}
	return out
}

func lcsTable(a, n []string) [][]int {
	dp := make([][]int, len(a)+1)
	for i := range dp {
		dp[i] = make([]int, len(n)+1)
	}
	for i := len(a) - 1; i >= 0; i-- {
		for j := len(n) - 1; j >= 0; j-- {
			if a[i] == n[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else if dp[i+1][j] >= dp[i][j+1] {
				dp[i][j] = dp[i+1][j]
			} else {
				dp[i][j] = dp[i][j+1]
			}
		}
	}
	return dp
}
