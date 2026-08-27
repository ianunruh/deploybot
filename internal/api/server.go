package api

import (
	"net/http"

	"github.com/ianunruh/deploybot/internal/catalog"
	"github.com/ianunruh/deploybot/internal/ops"
	"github.com/ianunruh/deploybot/internal/release"
)

type Server struct {
	Release *release.Service
	Catalog *catalog.Catalog
	Ops     *ops.Service
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.healthz)
	mux.HandleFunc("GET /api/v1/deployables", s.list)
	mux.HandleFunc("GET /api/v1/updates", s.updates)
	mux.HandleFunc("GET /api/v1/history", s.listHistory)
	mux.HandleFunc("GET /api/v1/pause", s.getPause)
	mux.HandleFunc("POST /api/v1/pause", s.postPause)
	mux.HandleFunc("POST /api/v1/unpause", s.postUnpause)
	mux.HandleFunc("GET /api/v1/deployables/{name}", s.get)
	mux.HandleFunc("GET /api/v1/deployables/{name}/images", s.images)
	mux.HandleFunc("GET /api/v1/deployables/{name}/history", s.history)
	mux.HandleFunc("GET /api/v1/deployables/{name}/workloads", s.workloads)
	mux.HandleFunc("GET /api/v1/deployables/{name}/workflows", s.workflows)
	mux.HandleFunc("GET /api/v1/deployables/{name}/changelog", s.changelog)
	mux.HandleFunc("GET /api/v1/deployables/{name}/diff", s.diff)
	mux.HandleFunc("GET /api/v1/deployables/{name}/reconcile", s.reconcileDiff)
	mux.HandleFunc("POST /api/v1/deployables/{name}/pin", s.pin)
	mux.HandleFunc("POST /api/v1/deployables/{name}/promote", s.promote)
	mux.HandleFunc("POST /api/v1/deployables/{name}/rollback", s.rollback)
	mux.HandleFunc("POST /api/v1/deployables/{name}/reconcile", s.reconcile)
	mux.HandleFunc("GET /api/v1/ops/catalog", s.opsCatalog)
	mux.HandleFunc("GET /api/v1/ops/executions", s.listOps)
	mux.HandleFunc("POST /api/v1/ops/executions", s.startOps)
	mux.HandleFunc("GET /api/v1/ops/executions/{id}", s.getOps)
	mux.HandleFunc("GET /api/v1/ops/executions/{id}/logs", s.opsLogs)
	return withJSON(mux)
}

func (s *Server) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) mutator(sync, wait *bool) *release.Service {
	return s.mutateWith(nil, sync, wait)
}

func (s *Server) mutateWith(r *http.Request, sync, wait *bool) *release.Service {
	svc := s.Release
	if sync != nil {
		svc = svc.WithSync(*sync)
	}
	if wait != nil {
		svc = svc.WithWait(*wait)
	}
	if a := actorFromRequest(r); a.Kind != "" {
		return svc.WithActor(a)
	}
	return svc
}
