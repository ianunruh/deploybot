package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

const DefaultPath = "deploybot.yaml"

// File is process config: structured non-secrets. Git and GitHub tokens stay in env.
type File struct {
	Addr       string             `yaml:"addr,omitempty"`
	SpecsDir   string             `yaml:"specsDir,omitempty"`
	OpsRepo    string             `yaml:"opsRepo,omitempty"`
	OpsRepoURL string             `yaml:"opsRepoURL,omitempty"`
	Apply      *bool              `yaml:"apply,omitempty"`
	Push       *bool              `yaml:"push,omitempty"`
	Sync       *bool              `yaml:"sync,omitempty"`
	AutoPin    *bool              `yaml:"autoPin,omitempty"`
	Clusters   map[string]Cluster `yaml:"clusters,omitempty"`
}

// Cluster is a named environment (homelab, prod) with per-cluster UIs.
type Cluster struct {
	Argo     Argo     `yaml:"argo,omitempty"`
	Headlamp Headlamp `yaml:"headlamp,omitempty"`
	Grafana  Grafana  `yaml:"grafana,omitempty"`
}

// Argo is a per-cluster Argo CD origin. URL is the UI. Application CRs are
// read and synced via kubeconfig (kubeContext defaults to the cluster name).
type Argo struct {
	URL         string `yaml:"url,omitempty"`
	KubeContext string `yaml:"kubeContext,omitempty"`
	Kubeconfig  string `yaml:"kubeconfig,omitempty"`
	Namespace   string `yaml:"namespace,omitempty"`
}

// Headlamp is the cluster's Headlamp UI. Path and query are built by callers.
type Headlamp struct {
	URL string `yaml:"url,omitempty"`
}

// Grafana is the cluster's Grafana UI. Path and query are built by callers.
// Logs is opt-in: Loki drilldown is only linked when true.
type Grafana struct {
	URL  string `yaml:"url,omitempty"`
	Logs bool   `yaml:"logs,omitempty"`
}

func Load(path string) (*File, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	var f File
	if err := yaml.Unmarshal(b, &f); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	if f.Clusters == nil {
		f.Clusters = map[string]Cluster{}
	}
	normalized := make(map[string]Cluster, len(f.Clusters))
	for name, cl := range f.Clusters {
		name = strings.ToLower(strings.TrimSpace(name))
		if name == "" {
			return nil, fmt.Errorf("parse config %s: empty cluster name", path)
		}
		cl.Argo.URL = trimURL(cl.Argo.URL)
		cl.Headlamp.URL = trimURL(cl.Headlamp.URL)
		cl.Grafana.URL = trimURL(cl.Grafana.URL)
		normalized[name] = cl
	}
	f.Clusters = normalized
	return &f, nil
}

func trimURL(s string) string {
	return strings.TrimRight(strings.TrimSpace(s), "/")
}

// ResolvePath picks the config file. explicit (usually --config) wins, then
// DEPLOYBOT_CONFIG, then ./deploybot.yaml if it exists. Empty path means none.
func ResolvePath(explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	if v := strings.TrimSpace(os.Getenv("DEPLOYBOT_CONFIG")); v != "" {
		return v, nil
	}
	if _, err := os.Stat(DefaultPath); err == nil {
		return DefaultPath, nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("stat %s: %w", DefaultPath, err)
	}
	return "", nil
}

func Open(explicit string) (*File, string, error) {
	path, err := ResolvePath(explicit)
	if err != nil {
		return nil, "", err
	}
	if path == "" {
		return &File{Clusters: map[string]Cluster{}}, "", nil
	}
	f, err := Load(path)
	if err != nil {
		return nil, path, err
	}
	return f, path, nil
}

func EnvOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

// EnvBool reports DEPLOYBOT_* mode flags. set is false when the var is missing
// or empty, so a YAML value can apply.
func EnvBool(key string) (value, set bool) {
	s, ok := os.LookupEnv(key)
	if !ok || s == "" {
		return false, false
	}
	return s == "1", true
}
