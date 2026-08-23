package cli

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ianunruh/deploybot/internal/render"
	"github.com/ianunruh/deploybot/internal/spec"
)

func runRender(args []string) error {
	fs := flag.NewFlagSet("render", flag.ContinueOnError)
	out := fs.String("out", "", "output directory (default: stdout paths)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("render: spec path required")
	}
	d, err := spec.Load(fs.Arg(0))
	if err != nil {
		return err
	}
	tree, err := render.Render(d)
	if err != nil {
		return err
	}
	if *out == "" {
		for _, p := range render.SortedPaths(tree) {
			fmt.Printf("=== %s ===\n%s\n", p, tree[p])
		}
		return nil
	}
	for _, p := range render.SortedPaths(tree) {
		fp := filepath.Join(*out, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(fp), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(fp, tree[p], 0o644); err != nil {
			return err
		}
	}
	_, _ = fmt.Fprintf(os.Stderr, "wrote %d files to %s\n", len(tree), *out)
	return nil
}
