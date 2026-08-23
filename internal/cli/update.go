package cli

import (
	"context"
	"flag"
	"fmt"

	"github.com/ianunruh/deploybot/internal/image"
	"github.com/ianunruh/deploybot/internal/release"
)

func runUpdate(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	flags := mutationFlags(fs)
	specs := fs.String("specs", "examples", "directory of deployable specs")
	if err := fs.Parse(args); err != nil {
		return err
	}

	var (
		svc  *release.Service
		name string
		err  error
	)
	if *flags.spec != "" {
		svc, name, err = serviceFromFlags(fs, flags)
		if err != nil {
			return err
		}
		return runUpdateOne(ctx, svc, name, false)
	}

	svc, err = serviceFromSpecsDir(fs, flags, *specs)
	if err != nil {
		return err
	}
	var first error
	for _, d := range svc.Catalog.List() {
		if !d.TracksRegistry() {
			continue
		}
		if err := runUpdateOne(ctx, svc, d.Metadata.Name, true); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func runUpdateOne(ctx context.Context, svc *release.Service, name string, requireAuto bool) error {
	d, err := svc.Catalog.Get(name)
	if err != nil {
		return err
	}
	if !d.TracksRegistry() {
		return fmt.Errorf("%s does not opt into registry tracking (spec.update)", name)
	}
	if svc.Images == nil {
		svc.Images = image.DefaultRegistry()
	}
	st, err := svc.CheckUpdate(ctx, name)
	if err != nil {
		return err
	}
	printUpdate(st)
	if st.Error != "" {
		return fmt.Errorf("%s", st.Error)
	}
	if requireAuto && st.Auto == "" {
		return nil
	}
	if !st.Stale {
		return nil
	}
	_, mut, err := svc.PinNewest(ctx, name)
	if err != nil {
		return err
	}
	printMutation(mut)
	return nil
}

func printUpdate(st release.UpdateStatus) {
	current := st.Current.Compact
	if current == "" {
		current = "-"
	}
	newest := "-"
	if st.Newest != nil {
		newest = st.Newest.Tag
		if newest == "" {
			newest = st.Newest.Ref
		}
	}
	state := "ok"
	if st.Error != "" {
		state = "error"
	} else if st.Stale {
		state = "STALE"
	}
	auto := st.Auto
	if auto == "" {
		auto = "manual"
	}
	fmt.Printf("%s  %s  %s  ->  %s  %s  %s\n", st.Name, st.Stage, current, newest, state, auto)
}
