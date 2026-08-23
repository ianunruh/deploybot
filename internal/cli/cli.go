package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/ianunruh/deploybot/internal/api"
	"github.com/ianunruh/deploybot/internal/argo"
	"github.com/ianunruh/deploybot/internal/catalog"
	"github.com/ianunruh/deploybot/internal/gitwrite"
	"github.com/ianunruh/deploybot/internal/release"
	"github.com/ianunruh/deploybot/internal/render"
	"github.com/ianunruh/deploybot/internal/spec"
)

const usage = `deploybot — small-scale release control plane

Usage:
  deploybot render [--out dir] <spec>
  deploybot pin --spec <file> --stage <name> --image <ref> [--repo dir] [--apply] [--sync]
  deploybot promote --spec <file> --from <stage> --to <stage> [--repo dir] [--apply] [--sync]
  deploybot serve [--addr host:port] [--specs dir] [--repo dir] [--apply] [--sync]
  deploybot version
`

func Run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		_, _ = fmt.Fprint(os.Stderr, usage)
		return fmt.Errorf("command required")
	}
	switch args[0] {
	case "version", "-v", "--version":
		fmt.Println("deploybot 0.0.0")
		return nil
	case "help", "-h", "--help":
		_, _ = fmt.Fprint(os.Stdout, usage)
		return nil
	case "render":
		return runRender(args[1:])
	case "pin":
		return runPin(ctx, args[1:])
	case "promote":
		return runPromote(ctx, args[1:])
	case "serve":
		return runServe(ctx, args[1:])
	default:
		_, _ = fmt.Fprint(os.Stderr, usage)
		return fmt.Errorf("unknown command %q", args[0])
	}
}

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

func runPin(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("pin", flag.ContinueOnError)
	specPath, repo, apply, sync := mutationFlags(fs)
	stage := fs.String("stage", "", "stage to pin")
	imageRef := fs.String("image", "", "image reference (tag and/or digest)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *specPath == "" || *stage == "" || *imageRef == "" {
		return fmt.Errorf("pin requires --spec, --stage, and --image")
	}
	svc, name, err := serviceFromSpec(*specPath, *repo, *apply, *sync)
	if err != nil {
		return err
	}
	mut, err := svc.Pin(ctx, name, *stage, *imageRef)
	if err != nil {
		return err
	}
	printMutation(mut)
	return nil
}

func runPromote(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("promote", flag.ContinueOnError)
	specPath, repo, apply, sync := mutationFlags(fs)
	from := fs.String("from", "", "source stage")
	to := fs.String("to", "", "destination stage")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *specPath == "" || *from == "" || *to == "" {
		return fmt.Errorf("promote requires --spec, --from, and --to")
	}
	svc, name, err := serviceFromSpec(*specPath, *repo, *apply, *sync)
	if err != nil {
		return err
	}
	mut, err := svc.Promote(ctx, name, *from, *to)
	if err != nil {
		return err
	}
	printMutation(mut)
	return nil
}

func runServe(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	addr := fs.String("addr", envOr("DEPLOYBOT_ADDR", "127.0.0.1:8080"), "listen address")
	specs := fs.String("specs", envOr("DEPLOYBOT_SPECS_DIR", "examples"), "directory of deployable specs")
	repo := fs.String("repo", os.Getenv("DEPLOYBOT_OPS_REPO"), "local ops git repo")
	apply := fs.Bool("apply", os.Getenv("DEPLOYBOT_APPLY") == "1", "commit mutations to the ops repo")
	sync := fs.Bool("sync", os.Getenv("DEPLOYBOT_SYNC") == "1", "sync Argo after commit")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cat, err := catalog.Load(*specs)
	if err != nil {
		return err
	}
	svc := &release.Service{
		Catalog: cat,
		OpsRepo: *repo,
		Apply:   *apply,
		Sync:    *sync,
		Author:  gitwrite.DefaultAuthor(),
		Argo:    argo.EndpointsFromEnv(),
		Wait:    5 * time.Minute,
	}
	h := (&api.Server{Release: svc, Catalog: cat}).Handler()
	srv := &http.Server{Addr: *addr, Handler: h}
	go func() {
		<-ctx.Done()
		shCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shCtx)
	}()
	slog.Info("api listening", "addr", *addr, "specs", *specs, "apply", *apply, "sync", *sync)
	err = srv.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func mutationFlags(fs *flag.FlagSet) (specPath, repo *string, apply, sync *bool) {
	specPath = fs.String("spec", "", "deployable spec YAML")
	repo = fs.String("repo", os.Getenv("DEPLOYBOT_OPS_REPO"), "local ops git repo")
	apply = fs.Bool("apply", false, "commit to the ops repo (never pushes)")
	sync = fs.Bool("sync", false, "sync Argo and wait healthy after commit")
	return specPath, repo, apply, sync
}

func serviceFromSpec(specPath, repo string, apply, sync bool) (*release.Service, string, error) {
	d, err := spec.Load(specPath)
	if err != nil {
		return nil, "", err
	}
	cat, err := catalog.Load(filepath.Dir(specPath))
	if err != nil {
		return nil, "", err
	}
	return &release.Service{
		Catalog: cat,
		OpsRepo: repo,
		Apply:   apply,
		Sync:    sync,
		Author:  gitwrite.DefaultAuthor(),
		Argo:    argo.EndpointsFromEnv(),
		Wait:    5 * time.Minute,
	}, d.Metadata.Name, nil
}

func printMutation(mut release.Mutation) {
	if mut.DryRun {
		fmt.Println("dry-run (pass --apply to commit locally)")
	}
	if mut.Commit != "" {
		fmt.Println("commit", mut.Commit)
	}
	if mut.Synced {
		fmt.Println("argo synced")
	}
	if mut.Diff != "" {
		fmt.Print(mut.Diff)
	}
}

func envOr(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}
