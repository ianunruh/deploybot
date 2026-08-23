// Package logx logs external I/O at debug with elapsed time.
package logx

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"time"
)

// Done records an external call at debug. elapsed is time since start.
func Done(ctx context.Context, msg string, start time.Time, err error, args ...any) {
	if ctx == nil {
		ctx = context.Background()
	}
	if !slog.Default().Enabled(ctx, slog.LevelDebug) {
		return
	}
	out := make([]any, 0, len(args)+4)
	out = append(out, args...)
	out = append(out, "elapsed", time.Since(start))
	if err != nil {
		out = append(out, "err", err)
	}
	slog.DebugContext(ctx, msg, out...)
}

// Do is http.Client.Do plus a debug log of method, redacted URL, status, and elapsed.
func Do(name string, c *http.Client, req *http.Request) (*http.Response, error) {
	if c == nil {
		c = http.DefaultClient
	}
	ctx := context.Background()
	if req != nil {
		ctx = req.Context()
	}
	start := time.Now()
	resp, err := c.Do(req)
	args := []any{"method", req.Method, "url", requestURL(req)}
	if resp != nil {
		args = append(args, "status", resp.StatusCode)
	}
	Done(ctx, name, start, err, args...)
	return resp, err
}

// RedactURL strips a password from a URL for logs.
func RedactURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" {
		return raw
	}
	return u.Redacted()
}

func requestURL(req *http.Request) string {
	if req == nil || req.URL == nil {
		return ""
	}
	return req.URL.Redacted()
}
