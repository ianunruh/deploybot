package ops

import (
	"encoding/json"
	"time"
)

const (
	PhasePending   = "Pending"
	PhaseRunning   = "Running"
	PhaseSucceeded = "Succeeded"
	PhaseFailed    = "Failed"
)

// Actor is who started an execution. Copied from the request JWT.
type Actor struct {
	Kind  string `json:"kind,omitempty"`
	ID    string `json:"id,omitempty"`
	Repo  string `json:"repo,omitempty"`
	Name  string `json:"name,omitempty"`
	Email string `json:"email,omitempty"`
}

// Request is the generic start envelope. Params is kind-specific JSON.
type Request struct {
	Kind    string          `json:"kind"`
	Cluster string          `json:"cluster"`
	DryRun  *bool           `json:"dryRun"`
	Ref     string          `json:"ref"`
	Params  json.RawMessage `json:"params"`
}

func (r Request) dryRun() bool {
	if r.DryRun == nil {
		return true
	}
	return *r.DryRun
}

// Execution is one Job, decoded from labels/annotations/status.
type Execution struct {
	ID        string          `json:"id"`
	Kind      string          `json:"kind"`
	Cluster   string          `json:"cluster"`
	Phase     string          `json:"phase"`
	DryRun    bool            `json:"dryRun"`
	Ref       string          `json:"ref,omitempty"`
	Summary   string          `json:"summary,omitempty"`
	Command   []string        `json:"command,omitempty"`
	Params    json.RawMessage `json:"params,omitempty"`
	Actor     Actor           `json:"actor,omitempty"`
	PodName   string          `json:"podName,omitempty"`
	Message   string          `json:"message,omitempty"`
	CreatedAt *time.Time      `json:"createdAt,omitempty"`
}

// Catalog is GET /api/v1/ops/catalog.
type Catalog struct {
	Kinds      []CatalogKind `json:"kinds"`
	Clusters   []string      `json:"clusters"`
	DefaultRef string        `json:"defaultRef,omitempty"`
	ImageSet   bool          `json:"imageSet"`
}

// CatalogKind is the public Kind (no function pointers).
type CatalogKind struct {
	Name    string  `json:"name"`
	Title   string  `json:"title"`
	WorkDir string  `json:"workDir,omitempty"`
	Fields  []Field `json:"fields"`
}

func catalogKinds() []CatalogKind {
	src := Kinds()
	out := make([]CatalogKind, 0, len(src))
	for _, k := range src {
		out = append(out, CatalogKind{
			Name:    k.Name,
			Title:   k.Title,
			WorkDir: k.WorkDir,
			Fields:  k.Fields,
		})
	}
	return out
}

func (s *Service) Catalog() Catalog {
	c := Catalog{
		Kinds:    catalogKinds(),
		Clusters: []string{},
	}
	if s == nil {
		return c
	}
	c.Clusters = s.clusterNames()
	c.DefaultRef = s.Config.ref()
	c.ImageSet = s.Config.Image != ""
	return c
}
