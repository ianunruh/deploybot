package api

import (
	"net/http"

	"github.com/ianunruh/deploybot/internal/catalog"
	"github.com/ianunruh/deploybot/internal/release"
)

type Server struct {
	Release *release.Service
	Catalog *catalog.Catalog
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.healthz)
	mux.HandleFunc("GET /api/v1/deployables", s.list)
	mux.HandleFunc("GET /api/v1/deployables/{name}", s.get)
	mux.HandleFunc("GET /api/v1/deployables/{name}/images", s.images)
	mux.HandleFunc("GET /api/v1/deployables/{name}/diff", s.diff)
	mux.HandleFunc("GET /api/v1/deployables/{name}/reconcile", s.reconcileDiff)
	mux.HandleFunc("POST /api/v1/deployables/{name}/pin", s.pin)
	mux.HandleFunc("POST /api/v1/deployables/{name}/promote", s.promote)
	mux.HandleFunc("POST /api/v1/deployables/{name}/reconcile", s.reconcile)
	return withJSON(mux)
}

func (s *Server) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) mutator(sync *bool) *release.Service {
	if sync == nil {
		return s.Release
	}
	return s.Release.WithSync(*sync)
}
