package api

import (
	"encoding/json"
	"net/http"
)

type rollbackRequest struct {
	Stage string `json:"stage"`
	Image string `json:"image"`
	Sync  *bool  `json:"sync"`
	Wait  *bool  `json:"wait"`
}

func (s *Server) rollback(w http.ResponseWriter, r *http.Request) {
	var req rollbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	mut, err := s.mutateWith(r, req.Sync, req.Wait).Rollback(r.Context(), r.PathValue("name"), req.Stage, req.Image)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, mut)
}
