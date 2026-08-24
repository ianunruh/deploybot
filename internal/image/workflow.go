package image

import (
	"context"
	"time"
)

// WorkflowRun is a GitHub Actions run on a source repo.
type WorkflowRun struct {
	ID        int64      `json:"id"`
	Name      string     `json:"name"`
	Title     string     `json:"title,omitempty"`
	Number    int        `json:"number"`
	Event     string     `json:"event,omitempty"`
	Status    string     `json:"status"`
	Branch    string     `json:"branch,omitempty"`
	SHA       string     `json:"sha,omitempty"`
	Actor     string     `json:"actor,omitempty"`
	URL       string     `json:"url,omitempty"`
	CommitURL string     `json:"commitURL,omitempty"`
	StartedAt *time.Time `json:"startedAt,omitempty"`
}

// WorkflowLookup lists recent GitHub Actions runs for a github.com repoURL.
type WorkflowLookup interface {
	ListWorkflowRuns(ctx context.Context, repoURL string, limit int) ([]WorkflowRun, error)
}
