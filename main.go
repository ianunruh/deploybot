package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"

	"github.com/ianunruh/deploybot/internal/cli"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := cli.Run(ctx, os.Args[1:]); err != nil {
		slog.Error("command failed", "err", err)
		os.Exit(1)
	}
}
