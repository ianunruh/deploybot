package kube

import (
	"gopkg.in/yaml.v3"
)

type kubeconfig struct {
	CurrentContext string         `yaml:"current-context"`
	Clusters       []namedCluster `yaml:"clusters"`
	Contexts       []namedContext `yaml:"contexts"`
	Users          []namedUser    `yaml:"users"`
}

type namedCluster struct {
	Name    string  `yaml:"name"`
	Cluster cluster `yaml:"cluster"`
}

type cluster struct {
	Server                   string `yaml:"server"`
	CertificateAuthorityData string `yaml:"certificate-authority-data"`
	CertificateAuthority     string `yaml:"certificate-authority"`
	InsecureSkipTLSVerify    bool   `yaml:"insecure-skip-tls-verify"`
}

type namedContext struct {
	Name    string     `yaml:"name"`
	Context contextRef `yaml:"context"`
}

type contextRef struct {
	Cluster   string `yaml:"cluster"`
	User      string `yaml:"user"`
	Namespace string `yaml:"namespace"`
}

type namedUser struct {
	Name string `yaml:"name"`
	User user   `yaml:"user"`
}

type user struct {
	Token                 string      `yaml:"token"`
	TokenFile             string      `yaml:"tokenFile"`
	ClientCertificateData string      `yaml:"client-certificate-data"`
	ClientCertificate     string      `yaml:"client-certificate"`
	ClientKeyData         string      `yaml:"client-key-data"`
	ClientKey             string      `yaml:"client-key"`
	Exec                  *execConfig `yaml:"exec"`
}

type execConfig struct {
	APIVersion         string       `yaml:"apiVersion"`
	Command            string       `yaml:"command"`
	Args               []string     `yaml:"args"`
	Env                []execEnvVar `yaml:"env"`
	ProvideClusterInfo bool         `yaml:"provideClusterInfo"`
	InteractiveMode    string       `yaml:"interactiveMode"`
}

type execEnvVar struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

func parseConfig(b []byte) (*kubeconfig, error) {
	var cfg kubeconfig
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *kubeconfig) context(name string) (contextRef, bool) {
	for _, n := range c.Contexts {
		if n.Name == name {
			return n.Context, true
		}
	}
	return contextRef{}, false
}

func (c *kubeconfig) cluster(name string) (cluster, bool) {
	for _, n := range c.Clusters {
		if n.Name == name {
			return n.Cluster, true
		}
	}
	return cluster{}, false
}

func (c *kubeconfig) user(name string) (user, bool) {
	for _, n := range c.Users {
		if n.Name == name {
			return n.User, true
		}
	}
	return user{}, false
}
