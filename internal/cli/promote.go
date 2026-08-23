package cli

import (
	"context"
	"flag"
	"fmt"
)

func runPromote(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("promote", flag.ContinueOnError)
	flags := mutationFlags(fs)
	from := fs.String("from", "", "source stage")
	to := fs.String("to", "", "destination stage")
	imageRef := fs.String("image", "", "image to pin on destination (default: current source pin)")
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
	mut, err := svc.Promote(ctx, name, *from, *to, *imageRef)
	if err != nil {
		return err
	}
	printMutation(mut)
	return nil
}
