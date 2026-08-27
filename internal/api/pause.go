package api

import (
	"encoding/json"
	"net/http"
)

type pauseRequest struct {
	Name   string `json:"name"`
	Stage  string `json:"stage"`
	Reason string `json:"reason"`
}

func (s *Server) getPause(w http.ResponseWriter, _ *http.Request) {
	if s.Release == nil {
		writeJSON(w, http.StatusOK, map[string]any{"pause": struct{}{}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"pause": s.Release.CurrentPause()})
}

func (s *Server) postPause(w http.ResponseWriter, r *http.Request) {
	var req pauseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	mut, err := s.mutateWith(r, nil, nil).SetPause(r.Context(), req.Name, req.Stage, req.Reason)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, mut)
}

func (s *Server) postUnpause(w http.ResponseWriter, r *http.Request) {
	var req pauseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	mut, err := s.mutateWith(r, nil, nil).ClearPause(r.Context(), req.Name, req.Stage)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, mut)
}
