package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/ianunruh/deploybot/internal/release"
)

func (s *Server) list(w http.ResponseWriter, r *http.Request) {
	type item struct {
		Name       string                 `json:"name"`
		Namespace  string                 `json:"namespace"`
		RepoURL    string                 `json:"repoURL,omitempty"`
		ProjectURL string                 `json:"projectURL,omitempty"`
		DeployedAt *time.Time             `json:"deployedAt,omitempty"`
		Flow       release.Flow           `json:"flow"`
		Stages     []release.StageStatus  `json:"stages"`
		Update     *release.UpdateSummary `json:"update,omitempty"`
	}
	var latest map[string]release.Live
	if s.Release != nil {
		latest = s.Release.Latest(r.Context())
	}
	var items []item
	for _, d := range s.Catalog.List() {
		live := latest[d.Metadata.Name]
		stages := live.Stages
		if len(stages) == 0 {
			for _, st := range d.Spec.Stages {
				ss := release.StageStatus{Name: st.Name, Hostname: st.Hostname}
				if s.Release != nil {
					ss.HeadlampURL, ss.GrafanaURL, ss.LogsURL = s.Release.Observability(st.Name, d.Spec.Namespace)
				}
				stages = append(stages, ss)
			}
		}
		flow := live.Flow
		if flow.Hops == nil {
			flow.Hops = []release.Hop{}
		}
		var upd *release.UpdateSummary
		if s.Release != nil {
			upd = s.Release.UpdateSummary(d)
		}
		items = append(items, item{
			Name:       d.Metadata.Name,
			Namespace:  d.Spec.Namespace,
			RepoURL:    d.Spec.Links.RepoURL,
			ProjectURL: d.Spec.Links.ProjectURL,
			DeployedAt: live.DeployedAt,
			Flow:       flow,
			Stages:     stages,
			Update:     upd,
		})
	}
	if items == nil {
		items = []item{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"deployables": items})
}

func (s *Server) updates(w http.ResponseWriter, r *http.Request) {
	if s.Release == nil {
		writeJSON(w, http.StatusOK, release.UpdateList{Updates: []release.UpdateStatus{}})
		return
	}
	writeJSON(w, http.StatusOK, s.Release.ListUpdates(r.Context()))
}

func (s *Server) get(w http.ResponseWriter, r *http.Request) {
	st, err := s.Release.Status(r.Context(), r.PathValue("name"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) history(w http.ResponseWriter, r *http.Request) {
	limit := 0
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "limit must be a non-negative integer"})
			return
		}
		limit = n
	}
	h, err := s.Release.History(r.Context(), r.PathValue("name"), limit)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, h)
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
	d, err := s.Release.Diff(r.Context(), r.PathValue("name"), q.Get("stage"), q.Get("image"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"diff": d})
}
