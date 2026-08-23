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
	"sync"
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
	syncArgo := fs.Bool("sync", false, "sync Argo after commit")
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
		sync:  pickBool(got["sync"], *syncArgo, "DEPLOYBOT_SYNC", file.Sync),
	}
	if s.push && !s.apply {
		return fmt.Errorf("--push requires --apply")
	}
	repoURL := pickString(false, "", "DEPLOYBOT_OPS_REPO_URL", file.OpsRepoURL)
	if repoURL != "" {
		if s.repo == "" {
			s.repo = filepath.Join(os.TempDir(), "deploybot-ops")
		}
		if err := gitwrite.Ensure(ctx, repoURL, s.repo); err != nil {
			return err
		}
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
	gh := &image.GitHub{Token: token, HTTPClient: &http.Client{Timeout: 20 * time.Second}}
	svc := &release.Service{
		Catalog: cat,
		OpsRepo: s.repo,
		Apply:   s.apply,
		Push:    s.push,
		Sync:    s.sync,
		Author:  gitwrite.DefaultAuthor(),
		Argo:    eps,
		Wait:    5 * time.Minute,
		Images:  gh,
		Commits: gh,
	}
	h := (&api.Server{Release: svc, Catalog: cat}).Handler()
	srv := &http.Server{Addr: s.addr, Handler: h}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var wg sync.WaitGroup
	wg.Go(func() { svc.WatchFlows(ctx) })
	wg.Go(func() {
		<-ctx.Done()
		shCtx, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		_ = srv.Shutdown(shCtx)
	})
	slog.Info("api listening", "addr", s.addr, "specs", s.specs, "config", path, "apply", s.apply, "push", s.push, "sync", s.sync, "github", tokenSrc)
	err = srv.ListenAndServe()
	cancel()
	wg.Wait()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
