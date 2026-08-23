package spec

import (
	"cmp"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	APIVersion = "deploybot.kcloud.io/v1alpha1"
	Kind       = "Deployable"
)

// Deployable is the git-side description of an app deploybot can render and promote.
type Deployable struct {
	APIVersion string   `yaml:"apiVersion"`
	Kind       string   `yaml:"kind"`
	Metadata   Metadata `yaml:"metadata"`
	Spec       Spec     `yaml:"spec"`
}

type Metadata struct {
	Name string `yaml:"name"`
}

type Spec struct {
	Namespace string   `yaml:"namespace"`
	Git       Git      `yaml:"git"`
	Argo      Argo     `yaml:"argo"`
	Image     Image    `yaml:"image"`
	Workload  Workload `yaml:"workload"`
	Route     Route    `yaml:"route"`
	Stages    []Stage  `yaml:"stages"`
}

type Git struct {
	RepoURL         string `yaml:"repoURL"`
	TargetRevision  string `yaml:"targetRevision,omitempty"`
	WorkloadPath    string `yaml:"workloadPath"`
	ApplicationPath string `yaml:"applicationPath"`
}

type Argo struct {
	Project           string `yaml:"project"`
	Name              string `yaml:"name,omitempty"`
	DestinationServer string `yaml:"destinationServer,omitempty"`
	CreateNamespace   bool   `yaml:"createNamespace,omitempty"`
}

type Image struct {
	Repository  string   `yaml:"repository"`
	Tag         string   `yaml:"tag,omitempty"`
	PullSecrets []string `yaml:"pullSecrets,omitempty"`
}

type Workload struct {
	Kind               string               `yaml:"kind"`
	Replicas           int                  `yaml:"replicas,omitempty"`
	ServiceAccountName string               `yaml:"serviceAccountName,omitempty"`
	ContainerName      string               `yaml:"containerName,omitempty"`
	ContainerPort      int                  `yaml:"containerPort"`
	PortName           string               `yaml:"portName,omitempty"`
	EnvFrom            EnvFrom              `yaml:"envFrom,omitempty"`
	Env                []EnvVar             `yaml:"env,omitempty"`
	Volumes            []Volume             `yaml:"volumes,omitempty"`
	Probes             Probes               `yaml:"probes,omitempty"`
	Resources          ResourceRequirements `yaml:"resources,omitempty"`
}

type EnvFrom struct {
	ConfigMaps []string `yaml:"configMaps,omitempty"`
	Secrets    []string `yaml:"secrets,omitempty"`
}

type EnvVar struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

type Volume struct {
	Name      string `yaml:"name"`
	MountPath string `yaml:"mountPath"`
	ReadOnly  bool   `yaml:"readOnly,omitempty"`
	ConfigMap string `yaml:"configMap,omitempty"`
	Secret    string `yaml:"secret,omitempty"`
	Optional  bool   `yaml:"optional,omitempty"`
}

type Probes struct {
	Path      string    `yaml:"path"`
	Port      string    `yaml:"port,omitempty"`
	Startup   HTTPProbe `yaml:"startup,omitempty"`
	Liveness  HTTPProbe `yaml:"liveness,omitempty"`
	Readiness HTTPProbe `yaml:"readiness,omitempty"`
}

type HTTPProbe struct {
	Path                string `yaml:"path,omitempty"`
	Port                string `yaml:"port,omitempty"`
	InitialDelaySeconds int    `yaml:"initialDelaySeconds,omitempty"`
	PeriodSeconds       int    `yaml:"periodSeconds,omitempty"`
	FailureThreshold    int    `yaml:"failureThreshold,omitempty"`
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

type Route struct {
	Port             int    `yaml:"port,omitempty"`
	Timeout          string `yaml:"timeout,omitempty"`
	GatewayNamespace string `yaml:"gatewayNamespace,omitempty"`
}

type Stage struct {
	Name     string     `yaml:"name"`
	Hostname string     `yaml:"hostname"`
	Gateway  GatewayRef `yaml:"gateway"`
	Volumes  []Volume   `yaml:"volumes,omitempty"`
}

type GatewayRef struct {
	Name        string `yaml:"name"`
	Namespace   string `yaml:"namespace,omitempty"`
	SectionName string `yaml:"sectionName,omitempty"`
	Kind        string `yaml:"kind,omitempty"`
	Group       string `yaml:"group,omitempty"`
}

func Load(path string) (*Deployable, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read spec %s: %w", path, err)
	}
	return Parse(b)
}

func Parse(b []byte) (*Deployable, error) {
	var d Deployable
	if err := yaml.Unmarshal(b, &d); err != nil {
		return nil, fmt.Errorf("parse spec: %w", err)
	}
	d.Default()
	if err := d.Validate(); err != nil {
		return nil, err
	}
	return &d, nil
}

func (d *Deployable) Default() {
	d.Spec.Git.TargetRevision = cmp.Or(d.Spec.Git.TargetRevision, "HEAD")
	d.Spec.Argo.Name = cmp.Or(d.Spec.Argo.Name, d.Metadata.Name)
	d.Spec.Argo.DestinationServer = cmp.Or(d.Spec.Argo.DestinationServer, "https://kubernetes.default.svc")
	d.Spec.Workload.Kind = cmp.Or(d.Spec.Workload.Kind, "Deployment")
	if d.Spec.Workload.Replicas == 0 {
		d.Spec.Workload.Replicas = 1
	}
	d.Spec.Workload.ContainerName = cmp.Or(d.Spec.Workload.ContainerName, "web")
	d.Spec.Workload.PortName = cmp.Or(d.Spec.Workload.PortName, "http")
	if d.Spec.Route.Port == 0 {
		d.Spec.Route.Port = d.Spec.Workload.ContainerPort
	}
	d.Spec.Route.GatewayNamespace = cmp.Or(d.Spec.Route.GatewayNamespace, "envoy-gateway-system")
	d.Spec.Workload.Probes.Port = cmp.Or(d.Spec.Workload.Probes.Port, d.Spec.Workload.PortName)
	for i := range d.Spec.Stages {
		st := &d.Spec.Stages[i]
		st.Gateway.Group = cmp.Or(st.Gateway.Group, "gateway.networking.k8s.io")
		st.Gateway.Kind = cmp.Or(st.Gateway.Kind, "Gateway")
		st.Gateway.Namespace = cmp.Or(st.Gateway.Namespace, d.Spec.Route.GatewayNamespace)
	}
}

func (d *Deployable) Validate() error {
	var errs []string
	if d.APIVersion != APIVersion {
		errs = append(errs, fmt.Sprintf("apiVersion must be %s", APIVersion))
	}
	if d.Kind != Kind {
		errs = append(errs, fmt.Sprintf("kind must be %s", Kind))
	}
	if d.Metadata.Name == "" {
		errs = append(errs, "metadata.name is required")
	}
	if d.Spec.Namespace == "" {
		errs = append(errs, "spec.namespace is required")
	}
	if d.Spec.Image.Repository == "" {
		errs = append(errs, "spec.image.repository is required")
	}
	if d.Spec.Git.WorkloadPath == "" {
		errs = append(errs, "spec.git.workloadPath is required")
	}
	if d.Spec.Git.ApplicationPath == "" {
		errs = append(errs, "spec.git.applicationPath is required")
	}
	if d.Spec.Argo.Project == "" {
		errs = append(errs, "spec.argo.project is required")
	}
	if d.Spec.Workload.Kind != "Deployment" {
		errs = append(errs, "spec.workload.kind must be Deployment")
	}
	if d.Spec.Workload.ContainerPort <= 0 {
		errs = append(errs, "spec.workload.containerPort is required")
	}
	if len(d.Spec.Stages) == 0 {
		errs = append(errs, "spec.stages must have at least one stage")
	}
	seen := make(map[string]struct{}, len(d.Spec.Stages))
	for _, st := range d.Spec.Stages {
		if st.Name == "" {
			errs = append(errs, "stage name is required")
			continue
		}
		if _, ok := seen[st.Name]; ok {
			errs = append(errs, fmt.Sprintf("duplicate stage %q", st.Name))
		}
		seen[st.Name] = struct{}{}
		if st.Hostname == "" {
			errs = append(errs, fmt.Sprintf("stage %s: hostname is required", st.Name))
		}
		if st.Gateway.Name == "" {
			errs = append(errs, fmt.Sprintf("stage %s: gateway.name is required", st.Name))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("invalid spec: %s", strings.Join(errs, "; "))
	}
	return nil
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

func (d *Deployable) ImageRef() string {
	if d.Spec.Image.Tag == "" {
		return d.Spec.Image.Repository
	}
	return d.Spec.Image.Repository + ":" + d.Spec.Image.Tag
}
