package logx

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func captureDebug(t *testing.T) *bytes.Buffer {
	t.Helper()
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	var buf bytes.Buffer
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	return &buf
}

func TestDoLogsMethodURLStatusElapsed(t *testing.T) {
	buf := captureDebug(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/x?q=1", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := Do("github", srv.Client(), req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status %d", resp.StatusCode)
	}
	got := buf.String()
	for _, want := range []string{"msg=github", "method=GET", "status=201", "elapsed=", "/x?q=1"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %s", want, got)
		}
	}
}

func TestDoLogsError(t *testing.T) {
	buf := captureDebug(t)
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://127.0.0.1:1/", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := Do("kube", &http.Client{Timeout: 50 * time.Millisecond}, req)
	if err == nil {
		if resp != nil {
			_ = resp.Body.Close()
		}
		t.Fatal("expected error")
	}
	got := buf.String()
	if !strings.Contains(got, "msg=kube") || !strings.Contains(got, "err=") || !strings.Contains(got, "elapsed=") {
		t.Fatalf("%s", got)
	}
	if strings.Contains(got, "status=") {
		t.Fatalf("status on transport error: %s", got)
	}
}

func TestDoRedactsPassword(t *testing.T) {
	buf := captureDebug(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.URL.User = url.UserPassword("u", "s3cret")
	resp, err := Do("dockerhub", srv.Client(), req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	got := buf.String()
	if strings.Contains(got, "s3cret") {
		t.Fatalf("password leaked: %s", got)
	}
	if !strings.Contains(got, "xxxxx") {
		t.Fatalf("expected redaction: %s", got)
	}
}

func TestDoneSilentAtInfo(t *testing.T) {
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	var buf bytes.Buffer
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})))
	Done(context.Background(), "git clone", time.Now(), nil, "dir", "/tmp")
	if buf.Len() != 0 {
		t.Fatalf("logged at info: %s", buf.String())
	}
}

func TestDoneIncludesErr(t *testing.T) {
	buf := captureDebug(t)
	Done(context.Background(), "git push", time.Now(), errors.New("denied"), "remote", "origin")
	got := buf.String()
	if !strings.Contains(got, "msg=\"git push\"") || !strings.Contains(got, "remote=origin") || !strings.Contains(got, "err=denied") {
		t.Fatalf("%s", got)
	}
}

func TestRedactURL(t *testing.T) {
	got := RedactURL("https://user:s3cret@example.com/a")
	if strings.Contains(got, "s3cret") {
		t.Fatalf("%s", got)
	}
	if !strings.Contains(got, "user") {
		t.Fatalf("%s", got)
	}
	if raw := "git@github.com:org/repo.git"; RedactURL(raw) != raw {
		t.Fatalf("scp %s", RedactURL(raw))
	}
}
