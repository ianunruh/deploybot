package argo

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPClientGetAndSync(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/applications/kmc", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer tok" {
			t.Errorf("auth %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"metadata": map[string]any{"name": "kmc"},
			"status": map[string]any{
				"health": map[string]any{"status": "Healthy", "message": "ok"},
				"sync":   map[string]any{"status": "Synced", "revision": "abc"},
			},
		})
	})
	mux.HandleFunc("POST /api/v1/applications/kmc/sync", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if string(body) == "" {
			t.Error("empty body")
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c := &HTTPClient{BaseURL: srv.URL, Token: "tok", HTTPClient: srv.Client()}
	st, err := c.Get(t.Context(), "kmc")
	if err != nil {
		t.Fatal(err)
	}
	if !st.Healthy() || st.Sync != "Synced" || st.Revision != "abc" {
		t.Fatalf("%+v", st)
	}
	if err := c.Sync(t.Context(), "kmc", true); err != nil {
		t.Fatal(err)
	}
}

func TestWaitHealthy(t *testing.T) {
	t.Parallel()
	f := NewFake()
	f.Set("kmc", Status{Health: "Progressing", Sync: "Synced"})
	go func() {
		time.Sleep(20 * time.Millisecond)
		f.Set("kmc", Status{Health: "Healthy", Sync: "Synced"})
	}()
	if err := WaitHealthy(t.Context(), f, "kmc", 5*time.Millisecond); err != nil {
		t.Fatal(err)
	}
}

func TestWaitDegraded(t *testing.T) {
	t.Parallel()
	f := NewFake()
	f.Set("kmc", Status{Health: "Degraded", Message: "crash"})
	if err := WaitHealthy(t.Context(), f, "kmc", time.Millisecond); err == nil {
		t.Fatal("expected error")
	}
}
