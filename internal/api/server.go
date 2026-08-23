package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

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
	mux.HandleFunc("GET /api/v1/deployables/{name}/sync", s.syncDiff)
	mux.HandleFunc("POST /api/v1/deployables/{name}/pin", s.pin)
	mux.HandleFunc("POST /api/v1/deployables/{name}/promote", s.promote)
	mux.HandleFunc("POST /api/v1/deployables/{name}/sync", s.syncManifests)
	return withJSON(mux)
}

func (s *Server) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) list(w http.ResponseWriter, r *http.Request) {
	type item struct {
		Name      string   `json:"name"`
		Namespace string   `json:"namespace"`
		Image     string   `json:"image"`
		Stages    []string `json:"stages"`
	}
	var items []item
	for _, d := range s.Catalog.List() {
		items = append(items, item{
			Name:      d.Metadata.Name,
			Namespace: d.Spec.Namespace,
			Image:     d.Spec.Image.Repository,
			Stages:    d.StageNames(),
		})
	}
	if items == nil {
		items = []item{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"deployables": items})
}

func (s *Server) get(w http.ResponseWriter, r *http.Request) {
	st, err := s.Release.Status(r.Context(), r.PathValue("name"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) images(w http.ResponseWriter, r *http.Request) {
	out, err := s.Release.ListImages(r.Context(), r.PathValue("name"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) diff(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	d, err := s.Release.Diff(r.PathValue("name"), q.Get("stage"), q.Get("image"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"diff": d})
}

type pinRequest struct {
	Stage string `json:"stage"`
	Image string `json:"image"`
}

func (s *Server) pin(w http.ResponseWriter, r *http.Request) {
	var req pinRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	mut, err := s.Release.Pin(r.Context(), r.PathValue("name"), req.Stage, req.Image)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, mut)
}

type promoteRequest struct {
	From string `json:"from"`
	To   string `json:"to"`
}

func (s *Server) promote(w http.ResponseWriter, r *http.Request) {
	var req promoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	mut, err := s.Release.Promote(r.Context(), r.PathValue("name"), req.From, req.To)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, mut)
}

func (s *Server) syncDiff(w http.ResponseWriter, r *http.Request) {
	stage := r.URL.Query().Get("stage")
	if stage == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "stage is required"})
		return
	}
	mut, err := s.Release.DiffSync(r.PathValue("name"), []string{stage})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, mut)
}

type syncRequest struct {
	Stage string `json:"stage"`
}

func (s *Server) syncManifests(w http.ResponseWriter, r *http.Request) {
	var req syncRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if req.Stage == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "stage is required"})
		return
	}
	mut, err := s.Release.SyncManifests(r.Context(), r.PathValue("name"), []string{req.Stage})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, mut)
}

func withJSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("write json", "err", err)
	}
}

func writeError(w http.ResponseWriter, err error) {
	status := http.StatusBadRequest
	if isNotFound(err) {
		status = http.StatusNotFound
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "unknown deployable") || strings.Contains(msg, "unknown stage")
}
