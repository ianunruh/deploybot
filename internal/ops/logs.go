package ops

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/ianunruh/deploybot/internal/kube"
)

// Logs returns the execution pod's log stream. follow waits for a pod and
// retries while the container is still creating. Caller must Close.
func (s *Service) Logs(ctx context.Context, cluster, id string, follow bool) (io.ReadCloser, error) {
	rest, err := s.rest(cluster)
	if err != nil {
		return nil, err
	}
	ex, err := s.Get(ctx, cluster, id)
	if err != nil {
		return nil, err
	}
	if !follow {
		if ex.PodName == "" {
			return io.NopCloser(emptyReader{}), nil
		}
		rc, err := rest.Stream(ctx, http.MethodGet, podLogPath(s.Config.ns(), ex.PodName, false))
		if err != nil {
			return nil, err
		}
		return rc, nil
	}
	pr, pw := io.Pipe()
	go func() {
		err := s.followLogs(ctx, rest, cluster, id, pw)
		_ = pw.CloseWithError(err)
	}()
	return pr, nil
}

func (s *Service) followLogs(ctx context.Context, rest *kube.REST, cluster, id string, w io.Writer) error {
	if _, err := io.WriteString(w, "waiting for pod...\n"); err != nil {
		return err
	}
	pod, err := s.waitPod(ctx, rest, s.Config.ns(), id)
	if err != nil {
		return err
	}
	if _, err := io.WriteString(w, "streaming "+pod+"\n"); err != nil {
		return err
	}
	tick := time.NewTicker(500 * time.Millisecond)
	defer tick.Stop()
	for {
		rc, err := rest.Stream(ctx, http.MethodGet, podLogPath(s.Config.ns(), pod, true))
		if err == nil {
			_, copyErr := io.Copy(w, rc)
			_ = rc.Close()
			return copyErr
		}
		var se *kube.StatusError
		if !errors.As(err, &se) || (se.Code != http.StatusBadRequest && se.Code != http.StatusNotFound) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
		}
		if next, err := currentPod(ctx, rest, s.Config.ns(), id); err == nil && next != "" {
			pod = next
		}
	}
}

type emptyReader struct{}

func (emptyReader) Read([]byte) (int, error) { return 0, io.EOF }
