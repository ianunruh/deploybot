package cli

import (
	"flag"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/ianunruh/deploybot/internal/argo"
	"github.com/ianunruh/deploybot/internal/catalog"
	"github.com/ianunruh/deploybot/internal/config"
	"github.com/ianunruh/deploybot/internal/gitwrite"
	"github.com/ianunruh/deploybot/internal/release"
	"github.com/ianunruh/deploybot/internal/spec"
)

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
	eps, err := argo.EndpointsFromConfig(file.Clusters)
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
		Catalog:  cat,
		OpsRepo:  s.repo,
		Apply:    s.apply,
		Push:     s.push,
		Sync:     s.sync,
		Author:   gitwrite.DefaultAuthor(),
		Argo:     eps,
		Clusters: file.Clusters,
		Wait:     5 * time.Minute,
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
