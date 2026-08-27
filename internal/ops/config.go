package ops

import (
	"strings"

	"github.com/ianunruh/deploybot/internal/config"
)

const (
	defaultNamespace      = "ops-ci"
	defaultServiceAccount = "deploybot-ops"
	defaultSecretName     = "ops-ci"
	defaultRef            = "main"
	defaultPullSecret     = "ghcr-auth"
	secretsMount          = "/var/run/ops-ci"
	containerName         = "ops"
	entrypoint            = "/usr/local/bin/ops-entrypoint"
)

// Config is how Jobs are created. Image must be a digest-pinned ref to POST.
type Config struct {
	Namespace      string
	Image          string
	RepoURL        string
	Ref            string
	ServiceAccount string
	SecretName     string
	PullSecret     string
}

// ConfigFromFile maps process YAML. Empty image is allowed (catalog still
// works; Start returns ErrNoImage).
func ConfigFromFile(file *config.File) Config {
	cfg := Config{
		Namespace:      defaultNamespace,
		ServiceAccount: defaultServiceAccount,
		SecretName:     defaultSecretName,
		PullSecret:     defaultPullSecret,
		Ref:            defaultRef,
		RepoURL:        "https://github.com/ianunruh/kcloud-ops",
	}
	if file == nil {
		return overlayEnv(cfg)
	}
	o := file.Ops
	if o.Namespace != "" {
		cfg.Namespace = o.Namespace
	}
	if o.Image != "" {
		cfg.Image = o.Image
	}
	if o.RepoURL != "" {
		cfg.RepoURL = o.RepoURL
	}
	if o.Ref != "" {
		cfg.Ref = o.Ref
	}
	if o.ServiceAccount != "" {
		cfg.ServiceAccount = o.ServiceAccount
	}
	if o.SecretName != "" {
		cfg.SecretName = o.SecretName
	}
	return overlayEnv(cfg)
}

func overlayEnv(cfg Config) Config {
	if v := config.EnvOr("DEPLOYBOT_OPS_NAMESPACE", ""); v != "" {
		cfg.Namespace = v
	}
	if v := config.EnvOr("DEPLOYBOT_OPS_IMAGE", ""); v != "" {
		cfg.Image = v
	}
	if v := config.EnvOr("DEPLOYBOT_OPS_REPO_URL", ""); v != "" {
		cfg.RepoURL = v
	}
	if v := config.EnvOr("DEPLOYBOT_OPS_REF", ""); v != "" {
		cfg.Ref = v
	}
	return cfg
}

func (c Config) ns() string {
	if c.Namespace != "" {
		return c.Namespace
	}
	return defaultNamespace
}

func (c Config) sa() string {
	if c.ServiceAccount != "" {
		return c.ServiceAccount
	}
	return defaultServiceAccount
}

func (c Config) secret() string {
	if c.SecretName != "" {
		return c.SecretName
	}
	return defaultSecretName
}

func (c Config) ref() string {
	if strings.TrimSpace(c.Ref) != "" {
		return strings.TrimSpace(c.Ref)
	}
	return defaultRef
}
