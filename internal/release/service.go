package release

import (
	"time"

	"github.com/ianunruh/deploybot/internal/argo"
	"github.com/ianunruh/deploybot/internal/catalog"
	"github.com/ianunruh/deploybot/internal/gitwrite"
	"github.com/ianunruh/deploybot/internal/image"
)

type Service struct {
	Catalog *catalog.Catalog
	OpsRepo string
	Apply   bool
	Push    bool
	Sync    bool
	Author  gitwrite.Author
	Argo    argo.Router
	Wait    time.Duration
	Images  image.Lister
}

type Mutation struct {
	DryRun bool     `json:"dryRun"`
	Commit string   `json:"commit,omitempty"`
	Pushed bool     `json:"pushed"`
	Ref    string   `json:"ref,omitempty"`
	Diff   string   `json:"diff"`
	Files  []string `json:"files"`
	Synced bool     `json:"synced"`
}

// WithSync returns a shallow copy with Argo sync forced on or off for one
// mutation. Sync cannot be turned on if the service has it disabled.
func (s *Service) WithSync(enabled bool) *Service {
	if s == nil || !s.Sync || enabled {
		return s
	}
	cp := *s
	cp.Sync = false
	return &cp
}

func (s *Service) author() gitwrite.Author {
	if s.Author.Name != "" {
		return s.Author
	}
	return gitwrite.DefaultAuthor()
}
