package release

import (
	"context"
	"time"

	"github.com/ianunruh/deploybot/internal/image"
)

const (
	workflowRunsTimeout = 2500 * time.Millisecond
	defaultWorkflowRuns = 10
)

// Workflows is recent GitHub Actions runs for a deployable's source repo.
type Workflows struct {
	URL   string              `json:"url,omitempty"`
	Runs  []image.WorkflowRun `json:"runs"`
	Error string              `json:"error,omitempty"`
}

func (s *Service) listWorkflows(ctx context.Context, repoURL string) *Workflows {
	if s == nil || s.Actions == nil {
		return nil
	}
	if _, _, ok := image.ParseGitHubRepo(repoURL); !ok {
		return nil
	}
	out := &Workflows{
		URL:  image.GitHubActionsURL(repoURL),
		Runs: []image.WorkflowRun{},
	}
	ctx, cancel := context.WithTimeout(ctx, workflowRunsTimeout)
	defer cancel()
	runs, err := s.Actions.ListWorkflowRuns(ctx, repoURL, defaultWorkflowRuns)
	if err != nil {
		out.Error = err.Error()
		return out
	}
	if runs != nil {
		out.Runs = runs
	}
	return out
}
