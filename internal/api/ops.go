package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/ianunruh/deploybot/internal/ops"
)

func (s *Server) opsCatalog(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.opsService().Catalog())
}

func (s *Server) listOps(w http.ResponseWriter, r *http.Request) {
	list, err := s.opsService().List(r.Context(), r.URL.Query().Get("kind"), r.URL.Query().Get("cluster"))
	if err != nil {
		writeError(w, err)
		return
	}
	if list == nil {
		list = []ops.Execution{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"executions": list})
}

func (s *Server) getOps(w http.ResponseWriter, r *http.Request) {
	ex, err := s.opsService().Get(r.Context(), r.URL.Query().Get("cluster"), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ex)
}

type opsStartRequest struct {
	Kind    string          `json:"kind"`
	Cluster string          `json:"cluster"`
	DryRun  *bool           `json:"dryRun"`
	Ref     string          `json:"ref"`
	Params  json.RawMessage `json:"params"`
}

func (s *Server) startOps(w http.ResponseWriter, r *http.Request) {
	var req opsStartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	ex, err := s.opsMutator(r).Start(r.Context(), ops.Request{
		Kind:    req.Kind,
		Cluster: req.Cluster,
		DryRun:  req.DryRun,
		Ref:     req.Ref,
		Params:  req.Params,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ex)
}

func (s *Server) opsLogs(w http.ResponseWriter, r *http.Request) {
	follow := r.URL.Query().Get("follow") == "1" || r.URL.Query().Get("follow") == "true"
	rc, err := s.opsService().Logs(r.Context(), r.URL.Query().Get("cluster"), r.PathValue("id"), follow)
	if err != nil {
		writeError(w, err)
		return
	}
	defer func() { _ = rc.Close() }()
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	flusher, _ := w.(http.Flusher)
	buf := make([]byte, 4096)
	for {
		n, err := rc.Read(buf)
		if n > 0 {
			if _, werr := w.Write(buf[:n]); werr != nil {
				return
			}
			if flusher != nil {
				flusher.Flush()
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			return
		}
	}
}

func (s *Server) opsService() *ops.Service {
	if s == nil || s.Ops == nil {
		return &ops.Service{}
	}
	return s.Ops
}

func (s *Server) opsMutator(r *http.Request) *ops.Service {
	svc := s.opsService()
	a := actorFromRequest(r)
	if a.Kind == "" {
		return svc
	}
	return svc.WithActor(ops.Actor{
		Kind:  a.Kind,
		ID:    a.ID,
		Repo:  a.Repo,
		Name:  a.Name,
		Email: a.Email,
	})
}
