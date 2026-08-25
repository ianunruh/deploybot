package cli

import (
	"context"
	"flag"
	"fmt"
)

func runRollback(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("rollback", flag.ContinueOnError)
	flags := mutationFlags(fs)
	stage := fs.String("stage", "", "stage to roll back")
	imageRef := fs.String("image", "", "previous image reference (tag and/or digest)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *flags.spec == "" || *stage == "" || *imageRef == "" {
		return fmt.Errorf("rollback requires --spec, --stage, and --image")
	}
	svc, name, err := serviceFromFlags(fs, flags)
	if err != nil {
		return err
	}
	mut, err := svc.Rollback(ctx, name, *stage, *imageRef)
	if err != nil {
		return err
	}
	printMutation(mut)
	return nil
}
