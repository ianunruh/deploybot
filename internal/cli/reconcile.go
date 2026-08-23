package cli

import (
	"context"
	"flag"
	"fmt"
)

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
