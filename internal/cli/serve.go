package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/ianunruh/deploybot/internal/api"
	"github.com/ianunruh/deploybot/internal/argo"
	"github.com/ianunruh/deploybot/internal/catalog"
	"github.com/ianunruh/deploybot/internal/config"
	"github.com/ianunruh/deploybot/internal/gitwrite"
	"github.com/ianunruh/deploybot/internal/image"
	"github.com/ianunruh/deploybot/internal/release"
)

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
