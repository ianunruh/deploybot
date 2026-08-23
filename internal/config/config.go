package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

const DefaultPath = "deploybot.yaml"

// File is process config: structured non-secrets. Tokens live in env or tokenFile.
type File struct {
	Addr     string          `yaml:"addr,omitempty"`
	SpecsDir string          `yaml:"specsDir,omitempty"`
	OpsRepo  string          `yaml:"opsRepo,omitempty"`
	Apply    *bool           `yaml:"apply,omitempty"`
	Push     *bool           `yaml:"push,omitempty"`
	Sync     *bool           `yaml:"sync,omitempty"`
	Argo     map[string]Argo `yaml:"argo,omitempty"`
}

// Argo is a per-stage Argo CD origin. URL is API and UI base
// (https://argocd.k8s.kcloud.zone).
type Argo struct {
	URL       string `yaml:"url,omitempty"`
	TokenFile string `yaml:"tokenFile,omitempty"`
	TokenEnv  string `yaml:"tokenEnv,omitempty"`
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
	if f.Argo == nil {
		f.Argo = map[string]Argo{}
	}
	normalized := make(map[string]Argo, len(f.Argo))
	for name, st := range f.Argo {
		name = strings.ToLower(strings.TrimSpace(name))
		if name == "" {
			return nil, fmt.Errorf("parse config %s: empty argo stage name", path)
		}
		if st.URL != "" {
			st.URL = strings.TrimRight(st.URL, "/")
		}
		normalized[name] = st
	}
	f.Argo = normalized
	return &f, nil
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
		return &File{Argo: map[string]Argo{}}, "", nil
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
