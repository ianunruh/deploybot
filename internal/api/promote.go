package api

import (
	"encoding/json"
	"net/http"
)

type promoteRequest struct {
	From string `json:"from"`
	To   string `json:"to"`
	Sync *bool  `json:"sync"`
}

func (s *Server) promote(w http.ResponseWriter, r *http.Request) {
	var req promoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	mut, err := s.mutator(req.Sync).Promote(r.Context(), r.PathValue("name"), req.From, req.To)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, mut)
}
