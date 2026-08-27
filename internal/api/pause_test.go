package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/ianunruh/deploybot/internal/catalog"
	"github.com/ianunruh/deploybot/internal/gitwrite"
	"github.com/ianunruh/deploybot/internal/release"
)

func TestPauseHTTP(t *testing.T) {
	t.Parallel()
	_, file, _, _ := runtime.Caller(0)
	cat, err := catalog.Load(filepath.Join(filepath.Dir(file), "..", "..", "examples"))
	if err != nil {
		t.Fatal(err)
	}

	s := &Server{Catalog: cat, Release: &release.Service{Catalog: cat}}
	srv := httptest.NewServer(s.Handler())
	t.Cleanup(srv.Close)

	res, err := http.Get(srv.URL + "/api/v1/pause")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		t.Fatal(res.Status)
	}

	body, err := json.Marshal(map[string]string{"stage": "delta"})
	if err != nil {
		t.Fatal(err)
	}
	bad, err := http.Post(srv.URL+"/api/v1/pause", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = bad.Body.Close() }()
	if bad.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown stage %s", bad.Status)
	}

	dir := initPauseRepo(t, "all:\n  at: 2026-08-26T18:00:00Z\n  by: ian\n  reason: freeze\n")
	apply := &Server{
		Catalog: cat,
		Release: &release.Service{
			Catalog: cat,
			OpsRepo: dir,
			Apply:   true,
			Author:  gitwrite.Author{Name: "t", Email: "t@t"},
		},
	}
	asrv := httptest.NewServer(apply.Handler())
	t.Cleanup(asrv.Close)

	got, err := http.Get(asrv.URL + "/api/v1/pause")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = got.Body.Close() }()
	var pause struct {
		Pause struct {
			All *struct {
				Reason string `json:"reason"`
			} `json:"all"`
		} `json:"pause"`
	}
	if err := json.NewDecoder(got.Body).Decode(&pause); err != nil {
		t.Fatal(err)
	}
	if pause.Pause.All == nil || pause.Pause.All.Reason != "freeze" {
		t.Fatalf("%+v", pause)
	}

	pin, err := json.Marshal(map[string]string{
		"stage": "homelab",
		"image": "ghcr.io/ianunruh/kmc@sha256:abc",
	})
	if err != nil {
		t.Fatal(err)
	}
	ciPin, err := http.NewRequestWithContext(
		t.Context(), http.MethodPost, asrv.URL+"/api/v1/deployables/kmc/pin", bytes.NewReader(pin),
	)
	if err != nil {
		t.Fatal(err)
	}
	ciPin.Header.Set("Content-Type", "application/json")
	ciPin.Header.Set("Authorization", "Bearer "+unsignedJWT(map[string]any{
		"iss":        "https://token.actions.githubusercontent.com",
		"actor":      "ianunruh",
		"repository": "ianunruh/kmc",
	}))
	blocked, err := http.DefaultClient.Do(ciPin)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = blocked.Body.Close() }()
	if blocked.StatusCode != http.StatusConflict {
		t.Fatalf("ci pin status %s", blocked.Status)
	}
	var errBody map[string]string
	if err := json.NewDecoder(blocked.Body).Decode(&errBody); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(errBody["error"], "paused") {
		t.Fatalf("%+v", errBody)
	}

	manual, err := http.Post(asrv.URL+"/api/v1/deployables/kmc/pin", "application/json", bytes.NewReader(pin))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = manual.Body.Close() }()
	if manual.StatusCode != http.StatusOK {
		t.Fatalf("manual pin while paused %s", manual.Status)
	}
}

func initPauseRepo(t *testing.T, pauseYAML string) string {
	t.Helper()
	dir := t.TempDir()
	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	fp := filepath.Join(dir, release.PausePath)
	if err := os.MkdirAll(filepath.Dir(fp), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fp, []byte(pauseYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := wt.Add(release.PausePath); err != nil {
		t.Fatal(err)
	}
	if _, err := wt.Commit("init", &git.CommitOptions{
		Author: &object.Signature{Name: "t", Email: "t@t", When: time.Now()},
	}); err != nil {
		t.Fatal(err)
	}
	return dir
}
