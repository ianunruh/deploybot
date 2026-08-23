package spec

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

type Workload struct {
	Kind               string               `yaml:"kind"`
	Replicas           int                  `yaml:"replicas,omitempty"`
	ServiceAccountName string               `yaml:"serviceAccountName,omitempty"`
	ContainerName      string               `yaml:"containerName,omitempty"`
	ContainerPort      int                  `yaml:"containerPort"`
	PortName           string               `yaml:"portName,omitempty"`
	Probes             Probes               `yaml:"probes,omitempty"`
	Resources          ResourceRequirements `yaml:"resources,omitempty"`
}

type Probes struct {
	Path      string    `yaml:"path"`
	Port      Port      `yaml:"port,omitempty"`
	Startup   HTTPProbe `yaml:"startup,omitempty"`
	Liveness  HTTPProbe `yaml:"liveness,omitempty"`
	Readiness HTTPProbe `yaml:"readiness,omitempty"`
}

type HTTPProbe struct {
	Path                string `yaml:"path,omitempty"`
	Port                Port   `yaml:"port,omitempty"`
	InitialDelaySeconds int    `yaml:"initialDelaySeconds,omitempty"`
	PeriodSeconds       int    `yaml:"periodSeconds,omitempty"`
	FailureThreshold    int    `yaml:"failureThreshold,omitempty"`
}

// Port is a Kubernetes probe port: a named port (http) or a number (8081).
// YAML accepts either a string or an integer scalar.
type Port string

func (p *Port) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind != yaml.ScalarNode {
		return fmt.Errorf("port must be a string or integer")
	}
	*p = Port(n.Value)
	return nil
}

func (p HTTPProbe) IsZero() bool {
	return p == HTTPProbe{}
}

type ResourceRequirements struct {
	Requests ResourceList `yaml:"requests,omitempty"`
	Limits   ResourceList `yaml:"limits,omitempty"`
}

type ResourceList struct {
	CPU    string `yaml:"cpu,omitempty"`
	Memory string `yaml:"memory,omitempty"`
}
