package api

import (
	"encoding/json"
	"net/http"
)

type promoteRequest struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Image string `json:"image"`
	Sync  *bool  `json:"sync"`
	Wait  *bool  `json:"wait"`
}

func (s *Server) promote(w http.ResponseWriter, r *http.Request) {
	var req promoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	mut, err := s.mutateWith(r, req.Sync, req.Wait).Promote(r.Context(), r.PathValue("name"), req.From, req.To, req.Image)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, mut)
}
