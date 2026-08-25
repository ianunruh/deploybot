package cli

import (
	"context"
	"fmt"
	"os"
)

const usage = `deploybot — Easy staged deploys across Kubernetes clusters

Usage:
  deploybot render [--out dir] <spec>
  deploybot pin --spec <file> --stage <name> --image <ref> [--config file] [--repo dir] [--apply] [--push] [--sync]
  deploybot promote --spec <file> --from <stage> --to <stage> [--image <ref>] [--config file] [--repo dir] [--apply] [--push] [--sync]
  deploybot rollback --spec <file> --stage <name> --image <ref> [--config file] [--repo dir] [--apply] [--push] [--sync]
  deploybot reconcile --spec <file> [--stage name]... [--config file] [--repo dir] [--apply] [--push] [--sync]
  deploybot update [--spec file] [--specs dir] [--config file] [--repo dir] [--apply] [--push] [--sync]
  deploybot serve [--config file] [--addr host:port] [--specs dir] [--repo dir] [--apply] [--push] [--sync] [--auto-pin]
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
	case "rollback":
		return runRollback(ctx, args[1:])
	case "reconcile":
		return runReconcile(ctx, args[1:])
	case "update":
		return runUpdate(ctx, args[1:])
	case "serve":
		return runServe(ctx, args[1:])
	default:
		_, _ = fmt.Fprint(os.Stderr, usage)
		return fmt.Errorf("unknown command %q", args[0])
	}
}
