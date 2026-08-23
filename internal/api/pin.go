package api

import (
	"encoding/json"
	"net/http"
)

type pinRequest struct {
	Stage string `json:"stage"`
	Image string `json:"image"`
	Sync  *bool  `json:"sync"`
}

func (s *Server) pin(w http.ResponseWriter, r *http.Request) {
	var req pinRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	mut, err := s.mutator(req.Sync).Pin(r.Context(), r.PathValue("name"), req.Stage, req.Image)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, mut)
}
