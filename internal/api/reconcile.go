package api

import (
	"encoding/json"
	"net/http"
)

func (s *Server) reconcileDiff(w http.ResponseWriter, r *http.Request) {
	stage := r.URL.Query().Get("stage")
	if stage == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "stage is required"})
		return
	}
	mut, err := s.Release.DiffReconcile(r.Context(), r.PathValue("name"), []string{stage})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, mut)
}

type reconcileRequest struct {
	Stage string `json:"stage"`
	Sync  *bool  `json:"sync"`
}

func (s *Server) reconcile(w http.ResponseWriter, r *http.Request) {
	var req reconcileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if req.Stage == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "stage is required"})
		return
	}
	mut, err := s.mutator(req.Sync).Reconcile(r.Context(), r.PathValue("name"), []string{req.Stage})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, mut)
}
