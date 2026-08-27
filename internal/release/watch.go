package release

import (
	"context"
	"log/slog"
	"time"
)

func (s *Service) WatchFlows(ctx context.Context) {
	if s == nil || !s.Apply {
		return
	}
	every := s.FlowEvery
	if every <= 0 {
		every = 15 * time.Second
	}
	s.ReconcileFlows(ctx)
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.ReconcileFlows(ctx)
		}
	}
}

func (s *Service) ReconcileFlows(ctx context.Context) {
	if s == nil || s.Catalog == nil || !s.Apply {
		return
	}
	if ctx.Err() != nil {
		return
	}
	if err := s.syncRepo(ctx); err != nil {
		slog.Warn("flow reconcile pull", "err", err)
		return
	}
	for _, d := range s.Catalog.List() {
		if ctx.Err() != nil {
			return
		}
		st, err := s.Status(ctx, d.Metadata.Name)
		if err != nil {
			slog.Warn("flow status", "deployable", d.Metadata.Name, "err", err)
			continue
		}
		for _, hop := range st.Flow.Hops {
			if hop.State != HopReady || hop.SourceImage == "" {
				continue
			}
			slog.Info("auto-promote", "deployable", d.Metadata.Name, "from", hop.From, "to", hop.To, "image", hop.SourceImage)
			if _, err := s.WithActor(ActorAutoPromote()).Promote(ctx, d.Metadata.Name, hop.From, hop.To, hop.SourceImage); err != nil {
				slog.Warn("auto-promote", "deployable", d.Metadata.Name, "from", hop.From, "to", hop.To, "err", err)
			}
		}
	}
}
