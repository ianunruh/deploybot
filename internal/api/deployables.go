package api

import (
	"net/http"

	"github.com/ianunruh/deploybot/internal/release"
)

func (s *Server) list(w http.ResponseWriter, r *http.Request) {
	type itemStage struct {
		Name        string `json:"name"`
		HeadlampURL string `json:"headlampURL,omitempty"`
		GrafanaURL  string `json:"grafanaURL,omitempty"`
		LogsURL     string `json:"logsURL,omitempty"`
	}
	type item struct {
		Name       string      `json:"name"`
		Namespace  string      `json:"namespace"`
		Image      string      `json:"image"`
		Stages     []string    `json:"stages"`
		RepoURL    string      `json:"repoURL,omitempty"`
		ProjectURL string      `json:"projectURL,omitempty"`
		StageLinks []itemStage `json:"stageLinks,omitempty"`
	}
	var items []item
	for _, d := range s.Catalog.List() {
		var links []itemStage
		for _, name := range d.StageNames() {
			st := itemStage{Name: name}
			st.HeadlampURL, st.GrafanaURL, st.LogsURL = release.ObservabilityURLs(name, d.Spec.Namespace)
			links = append(links, st)
		}
		items = append(items, item{
			Name:       d.Metadata.Name,
			Namespace:  d.Spec.Namespace,
			Image:      d.Spec.Image.Repository,
			Stages:     d.StageNames(),
			RepoURL:    d.Spec.Links.RepoURL,
			ProjectURL: d.Spec.Links.ProjectURL,
			StageLinks: links,
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
