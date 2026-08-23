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
	"strings"
	"time"

	"github.com/ianunruh/deploybot/internal/api"
	"github.com/ianunruh/deploybot/internal/argo"
	"github.com/ianunruh/deploybot/internal/catalog"
	"github.com/ianunruh/deploybot/internal/config"
	"github.com/ianunruh/deploybot/internal/gitwrite"
	"github.com/ianunruh/deploybot/internal/image"
	"github.com/ianunruh/deploybot/internal/release"
	"github.com/ianunruh/deploybot/internal/render"
	"github.com/ianunruh/deploybot/internal/spec"
)

const usage = `deploybot — small-scale release control plane

Usage:
  deploybot render [--out dir] <spec>
  deploybot pin --spec <file> --stage <name> --image <ref> [--config file] [--repo dir] [--apply] [--push] [--sync]
  deploybot promote --spec <file> --from <stage> --to <stage> [--config file] [--repo dir] [--apply] [--push] [--sync]
  deploybot reconcile --spec <file> [--stage name]... [--config file] [--repo dir] [--apply] [--push] [--sync]
  deploybot serve [--config file] [--addr host:port] [--specs dir] [--repo dir] [--apply] [--push] [--sync]
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
	case "reconcile":
		return runReconcile(ctx, args[1:])
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
	flags := mutationFlags(fs)
	stage := fs.String("stage", "", "stage to pin")
	imageRef := fs.String("image", "", "image reference (tag and/or digest)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *flags.spec == "" || *stage == "" || *imageRef == "" {
		return fmt.Errorf("pin requires --spec, --stage, and --image")
	}
	svc, name, err := serviceFromFlags(fs, flags)
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
	flags := mutationFlags(fs)
	from := fs.String("from", "", "source stage")
	to := fs.String("to", "", "destination stage")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *flags.spec == "" || *from == "" || *to == "" {
		return fmt.Errorf("promote requires --spec, --from, and --to")
	}
	svc, name, err := serviceFromFlags(fs, flags)
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

func runReconcile(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("reconcile", flag.ContinueOnError)
	flags := mutationFlags(fs)
	var stages stringList
	fs.Var(&stages, "stage", "stage to write (repeatable or comma-separated; default: all)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *flags.spec == "" {
		return fmt.Errorf("reconcile requires --spec")
	}
	svc, name, err := serviceFromFlags(fs, flags)
	if err != nil {
		return err
	}
	mut, err := svc.Reconcile(ctx, name, []string(stages))
	if err != nil {
		return err
	}
	printMutation(mut)
	return nil
}

func runServe(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	cfgPath := configFlag(fs)
	addr := fs.String("addr", "127.0.0.1:8080", "listen address")
	specs := fs.String("specs", "examples", "directory of deployable specs")
	repo := fs.String("repo", "", "local ops git repo")
	apply := fs.Bool("apply", false, "commit mutations to the ops repo")
	push := fs.Bool("push", false, "push the current branch after commit (requires --apply; never force-pushes)")
	sync := fs.Bool("sync", false, "sync Argo after commit")
	if err := fs.Parse(args); err != nil {
		return err
	}
	file, path, err := config.Open(*cfgPath)
	if err != nil {
		return err
	}
	got := visited(fs)
	s := settings{
		addr:  pickString(got["addr"], *addr, "DEPLOYBOT_ADDR", file.Addr),
		specs: pickString(got["specs"], *specs, "DEPLOYBOT_SPECS_DIR", file.SpecsDir),
		repo:  pickString(got["repo"], *repo, "DEPLOYBOT_OPS_REPO", file.OpsRepo),
		apply: pickBool(got["apply"], *apply, "DEPLOYBOT_APPLY", file.Apply),
		push:  pickBool(got["push"], *push, "DEPLOYBOT_PUSH", file.Push),
		sync:  pickBool(got["sync"], *sync, "DEPLOYBOT_SYNC", file.Sync),
	}
	if s.push && !s.apply {
		return fmt.Errorf("--push requires --apply")
	}
	cat, err := catalog.Load(s.specs)
	if err != nil {
		return err
	}
	eps, err := argo.EndpointsFromConfig(file.Argo)
	if err != nil {
		return err
	}
	token, tokenSrc := image.ResolveToken()
	svc := &release.Service{
		Catalog: cat,
		OpsRepo: s.repo,
		Apply:   s.apply,
		Push:    s.push,
		Sync:    s.sync,
		Author:  gitwrite.DefaultAuthor(),
		Argo:    eps,
		Wait:    5 * time.Minute,
		Images:  &image.GitHub{Token: token, HTTPClient: &http.Client{Timeout: 20 * time.Second}},
	}
	h := (&api.Server{Release: svc, Catalog: cat}).Handler()
	srv := &http.Server{Addr: s.addr, Handler: h}
	go func() {
		<-ctx.Done()
		shCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shCtx)
	}()
	slog.Info("api listening", "addr", s.addr, "specs", s.specs, "config", path, "apply", s.apply, "push", s.push, "sync", s.sync, "github", tokenSrc)
	err = srv.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

type mutFlags struct {
	spec   *string
	config *string
	repo   *string
	apply  *bool
	push   *bool
	sync   *bool
}

func mutationFlags(fs *flag.FlagSet) mutFlags {
	return mutFlags{
		spec:   fs.String("spec", "", "deployable spec YAML"),
		config: configFlag(fs),
		repo:   fs.String("repo", "", "local ops git repo"),
		apply:  fs.Bool("apply", false, "commit to the ops repo"),
		push:   fs.Bool("push", false, "push the current branch after commit (requires --apply; never force-pushes)"),
		sync:   fs.Bool("sync", false, "sync Argo and wait healthy after commit"),
	}
}

func configFlag(fs *flag.FlagSet) *string {
	return fs.String("config", "", "YAML config (default: DEPLOYBOT_CONFIG or ./deploybot.yaml)")
}

type settings struct {
	addr, specs, repo string
	apply, push, sync bool
}

func visited(fs *flag.FlagSet) map[string]bool {
	out := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { out[f.Name] = true })
	return out
}

func pickString(explicit bool, flagVal, envKey, yamlVal string) string {
	if explicit {
		return flagVal
	}
	if v := config.EnvOr(envKey, ""); v != "" {
		return v
	}
	if yamlVal != "" {
		return yamlVal
	}
	return flagVal
}

func pickBool(explicit bool, flagVal bool, envKey string, yamlVal *bool) bool {
	if explicit {
		return flagVal
	}
	if v, set := config.EnvBool(envKey); set {
		return v
	}
	if yamlVal != nil {
		return *yamlVal
	}
	return flagVal
}

func serviceFromFlags(fs *flag.FlagSet, flags mutFlags) (*release.Service, string, error) {
	file, _, err := config.Open(*flags.config)
	if err != nil {
		return nil, "", err
	}
	got := visited(fs)
	s := settings{
		repo:  pickString(got["repo"], *flags.repo, "DEPLOYBOT_OPS_REPO", file.OpsRepo),
		apply: pickBool(got["apply"], *flags.apply, "DEPLOYBOT_APPLY", file.Apply),
		push:  pickBool(got["push"], *flags.push, "DEPLOYBOT_PUSH", file.Push),
		sync:  pickBool(got["sync"], *flags.sync, "DEPLOYBOT_SYNC", file.Sync),
	}
	if s.push && !s.apply {
		return nil, "", fmt.Errorf("--push requires --apply")
	}
	eps, err := argo.EndpointsFromConfig(file.Argo)
	if err != nil {
		return nil, "", err
	}
	d, err := spec.Load(*flags.spec)
	if err != nil {
		return nil, "", err
	}
	cat, err := catalog.Load(filepath.Dir(*flags.spec))
	if err != nil {
		return nil, "", err
	}
	return &release.Service{
		Catalog: cat,
		OpsRepo: s.repo,
		Apply:   s.apply,
		Push:    s.push,
		Sync:    s.sync,
		Author:  gitwrite.DefaultAuthor(),
		Argo:    eps,
		Wait:    5 * time.Minute,
	}, d.Metadata.Name, nil
}

func printMutation(mut release.Mutation) {
	if mut.DryRun {
		fmt.Println("dry-run (pass --apply to commit locally; pass --apply --push to update the remote)")
	}
	if mut.Commit != "" {
		fmt.Println("commit", mut.Commit)
	}
	if mut.Pushed {
		fmt.Println("pushed", mut.Ref, mut.Commit)
	}
	if mut.Synced {
		fmt.Println("argo synced")
	}
	if mut.Diff != "" {
		fmt.Print(mut.Diff)
	}
}

type stringList []string

func (s *stringList) String() string { return strings.Join(*s, ",") }

func (s *stringList) Set(v string) error {
	n := 0
	for _, p := range strings.Split(v, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		*s = append(*s, p)
		n++
	}
	if n == 0 {
		return fmt.Errorf("empty stage")
	}
	return nil
}
