package ops

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"
)

// Field is one catalog form control. The console renders these; it does not
// special-case kinds.
type Field struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Title       string   `json:"title,omitempty"`
	Description string   `json:"description,omitempty"`
	Required    bool     `json:"required,omitempty"`
	Options     []string `json:"options,omitempty"`
	Suggestions []string `json:"suggestions,omitempty"`
	Keys        []string `json:"keys,omitempty"`
}

const (
	FieldMulti  = "multi"
	FieldString = "string"
	FieldMap    = "map"
	FieldBool   = "bool"
)

// Kind is one allowlisted operation. Shared Job/list/log machinery looks
// only at these fields — adding a kind is a new file, not a new API.
type Kind struct {
	Name           string
	Title          string
	WorkDir        string
	Deadline       time.Duration
	ServiceAccount string
	Fields         []Field

	Validate  func(cluster string, params json.RawMessage) error
	Argv      func(cluster string, dryRun bool, params json.RawMessage) ([]string, error)
	Summary   func(params json.RawMessage) string
	AvoidNode func(cluster string, params json.RawMessage) string
}

var registry []Kind

func register(k Kind) {
	if k.Name == "" {
		panic("ops kind: empty name")
	}
	if Lookup(k.Name) != nil {
		panic("ops kind: duplicate " + k.Name)
	}
	if k.WorkDir == "" {
		k.WorkDir = "deploys"
	}
	if k.Deadline <= 0 {
		k.Deadline = time.Hour
	}
	registry = append(registry, k)
}

// Lookup returns a registered kind or nil.
func Lookup(name string) *Kind {
	name = strings.TrimSpace(name)
	for i := range registry {
		if registry[i].Name == name {
			return &registry[i]
		}
	}
	return nil
}

// Kinds is the registered catalog in registration order.
func Kinds() []Kind {
	out := make([]Kind, len(registry))
	copy(out, registry)
	return out
}

func kindNames() []string {
	out := make([]string, 0, len(registry))
	for _, k := range registry {
		out = append(out, k.Name)
	}
	return out
}

func unknownKind(name string) error {
	return fmt.Errorf("unknown ops kind %q (have %s)", name, strings.Join(kindNames(), ", "))
}

func compactJSON(params json.RawMessage) string {
	if len(params) == 0 {
		return "{}"
	}
	var buf bytes.Buffer
	if err := json.Compact(&buf, params); err != nil {
		return strings.TrimSpace(string(params))
	}
	return buf.String()
}

func stringSlice(v any) []string {
	switch t := v.(type) {
	case nil:
		return nil
	case string:
		return splitList(t)
	case []string:
		out := make([]string, 0, len(t))
		for _, s := range t {
			s = strings.TrimSpace(s)
			if s != "" {
				out = append(out, s)
			}
		}
		return out
	case []any:
		out := make([]string, 0, len(t))
		for _, item := range t {
			s, ok := item.(string)
			if !ok {
				continue
			}
			s = strings.TrimSpace(s)
			if s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func requireKeys(raw map[string]any, allowed ...string) error {
	for k := range raw {
		if !slices.Contains(allowed, k) {
			return fmt.Errorf("unknown param %q", k)
		}
	}
	return nil
}
