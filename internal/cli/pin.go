package cli

import (
	"context"
	"flag"
	"fmt"
)

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
