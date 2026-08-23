package spec

import (
	"fmt"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	AfterHealthy  = "healthy"
	AfterBake     = "bake"
	AfterApproval = "approval"
)

type Stage struct {
	Name     string         `yaml:"name"`
	Hostname string         `yaml:"hostname"`
	Gateway  GatewayRef     `yaml:"gateway"`
	Promote  *PromotePolicy `yaml:"promote,omitempty"`
}

// PromotePolicy is how a digest enters this stage from an earlier one.
// Omit it to keep promote a console action with no auto-advance.
type PromotePolicy struct {
	From  string   `yaml:"from,omitempty"`
	After []string `yaml:"after"`
	Bake  Duration `yaml:"bake,omitempty"`
}

func (p PromotePolicy) Has(gate string) bool {
	for _, x := range p.After {
		if x == gate {
			return true
		}
	}
	return false
}

// AutoPromote is true when the reconciler should copy the source digest here
// once gates pass. Approval is always a human click.
func (p PromotePolicy) AutoPromote() bool {
	return !p.Has(AfterApproval)
}

// Duration is a Go duration string in YAML (30m, 1h).
type Duration time.Duration

func (d Duration) Duration() time.Duration { return time.Duration(d) }

func (d *Duration) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind != yaml.ScalarNode {
		return fmt.Errorf("duration must be a string")
	}
	v := strings.TrimSpace(n.Value)
	if v == "" {
		*d = 0
		return nil
	}
	parsed, err := time.ParseDuration(v)
	if err != nil {
		return fmt.Errorf("invalid duration %q", v)
	}
	if parsed < 0 {
		return fmt.Errorf("duration must be positive")
	}
	*d = Duration(parsed)
	return nil
}

type GatewayRef struct {
	Name        string `yaml:"name"`
	Namespace   string `yaml:"namespace,omitempty"`
	SectionName string `yaml:"sectionName,omitempty"`
	Kind        string `yaml:"kind,omitempty"`
	Group       string `yaml:"group,omitempty"`
}

func (d *Deployable) Stage(name string) (Stage, error) {
	for _, st := range d.Spec.Stages {
		if st.Name == name {
			return st, nil
		}
	}
	return Stage{}, fmt.Errorf("unknown stage %q", name)
}

func (d *Deployable) StageNames() []string {
	out := make([]string, len(d.Spec.Stages))
	for i, st := range d.Spec.Stages {
		out[i] = st.Name
	}
	return out
}

func (d *Deployable) BaseStage() Stage {
	return d.Spec.Stages[0]
}

// SourceStage is the overlay a promote into dest copies from.
func (d *Deployable) SourceStage(dest string) (string, error) {
	names := d.StageNames()
	idx := -1
	for i, n := range names {
		if n == dest {
			idx = i
			break
		}
	}
	if idx < 0 {
		return "", fmt.Errorf("unknown stage %q", dest)
	}
	st := d.Spec.Stages[idx]
	if st.Promote != nil && st.Promote.From != "" {
		return st.Promote.From, nil
	}
	if idx == 0 {
		return "", fmt.Errorf("no source stage for %q", dest)
	}
	return names[idx-1], nil
}
