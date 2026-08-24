package release

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ianunruh/deploybot/internal/image"
)

type fakeActions struct {
	repo string
	runs []image.WorkflowRun
	err  error
	hits int
}

func (f *fakeActions) ListWorkflowRuns(_ context.Context, repoURL string, limit int) ([]image.WorkflowRun, error) {
	f.hits++
	f.repo = repoURL
	if limit != defaultWorkflowRuns {
		return nil, fmt.Errorf("limit %d", limit)
	}
	return f.runs, f.err
}

func TestListWorkflowsSkipped(t *testing.T) {
	t.Parallel()
	if got := (*Service)(nil).listWorkflows(t.Context(), "https://github.com/ianunruh/kmc"); got != nil {
		t.Fatalf("nil service %+v", got)
	}
	if got := (&Service{}).listWorkflows(t.Context(), "https://github.com/ianunruh/kmc"); got != nil {
		t.Fatalf("no actions %+v", got)
	}
	act := &fakeActions{runs: []image.WorkflowRun{{ID: 1, Name: "Docker"}}}
	if got := (&Service{Actions: act}).listWorkflows(t.Context(), "https://gitlab.com/ianunruh/kmc"); got != nil {
		t.Fatalf("gitlab %+v", got)
	}
	if act.hits != 0 {
		t.Fatalf("gitlab hits %d", act.hits)
	}
}

func TestListWorkflowsGitHub(t *testing.T) {
	t.Parallel()
	started := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)
	act := &fakeActions{runs: []image.WorkflowRun{
		{ID: 101, Name: "Docker", Status: "in progress", Number: 42, StartedAt: &started},
		{ID: 100, Name: "Docker", Status: "success", Number: 41},
	}}
	got := (&Service{Actions: act}).listWorkflows(t.Context(), "https://github.com/ianunruh/kmc")
	if got == nil {
		t.Fatal("expected workflows")
	}
	if got.URL != "https://github.com/ianunruh/kmc/actions" {
		t.Fatalf("url %q", got.URL)
	}
	if got.Error != "" || len(got.Runs) != 2 || got.Runs[0].Status != "in progress" {
		t.Fatalf("%+v", got)
	}
	if act.repo != "https://github.com/ianunruh/kmc" || act.hits != 1 {
		t.Fatalf("lookup %+v", act)
	}
}

func TestListWorkflowsError(t *testing.T) {
	t.Parallel()
	act := &fakeActions{err: fmt.Errorf("github 403: nope")}
	got := (&Service{Actions: act}).listWorkflows(t.Context(), "https://github.com/ianunruh/kmc")
	if got == nil || got.Error == "" || got.URL == "" {
		t.Fatalf("%+v", got)
	}
	if len(got.Runs) != 0 {
		t.Fatalf("runs %+v", got.Runs)
	}
}

func TestStatusWorkflows(t *testing.T) {
	t.Parallel()
	act := &fakeActions{runs: []image.WorkflowRun{
		{ID: 101, Name: "Docker", Status: "failure", Number: 42},
	}}
	svc := &Service{Catalog: loadExamples(t), Actions: act}
	st, err := svc.Status(t.Context(), "kmc")
	if err != nil {
		t.Fatal(err)
	}
	if st.Workflows == nil || len(st.Workflows.Runs) != 1 || st.Workflows.Runs[0].ID != 101 {
		t.Fatalf("workflows %+v", st.Workflows)
	}
	if act.repo != "https://github.com/ianunruh/kmc" {
		t.Fatalf("repo %q", act.repo)
	}

	sonarr, err := svc.Status(t.Context(), "sonarr")
	if err != nil {
		t.Fatal(err)
	}
	if sonarr.Workflows == nil || sonarr.Workflows.URL != "https://github.com/linuxserver/docker-sonarr/actions" {
		t.Fatalf("sonarr %+v", sonarr.Workflows)
	}
}

func TestStatusOmitsWorkflowsWithoutActions(t *testing.T) {
	t.Parallel()
	svc := &Service{Catalog: loadExamples(t)}
	st, err := svc.Status(t.Context(), "kmc")
	if err != nil {
		t.Fatal(err)
	}
	if st.Workflows != nil {
		t.Fatalf("expected omit %+v", st.Workflows)
	}
}
