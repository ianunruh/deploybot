package spec

import (
	"cmp"
	"fmt"
	"os"
	"time"

	"github.com/ianunruh/deploybot/internal/image"
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
	Namespace string        `yaml:"namespace"`
	Links     Links         `yaml:"links,omitempty"`
	Git       Git           `yaml:"git"`
	Argo      Argo          `yaml:"argo"`
	Image     Image         `yaml:"image"`
	Update    *UpdatePolicy `yaml:"update,omitempty"`
	Workload  Workload      `yaml:"workload"`
	Route     Route         `yaml:"route"`
	Stages    []Stage       `yaml:"stages"`
}

// Links are optional URLs shown in the console. RepoURL is the app source
// (GitHub/GitLab). ProjectURL is a tracker or board (Trello, Linear, …).
// Source means we build that repo (main-<sha> pins, promote changelog).
type Links struct {
	RepoURL    string `yaml:"repoURL,omitempty"`
	ProjectURL string `yaml:"projectURL,omitempty"`
	Source     bool   `yaml:"source,omitempty"`
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

// UpdatePolicy opts a deployable into registry tracking. Presence of the
// block means deploybot compares the first-stage pin to the newest published
// image. Auto, if set, enrolls in scheduled first-stage pins.
type UpdatePolicy struct {
	Auto *Duration `yaml:"auto,omitempty"`
}

const MinAutoUpdate = Duration(time.Hour)

// TracksRegistry is true when spec.update is set.
func (d *Deployable) TracksRegistry() bool {
	return d != nil && d.Spec.Update != nil
}

// HasSourceCommits is true when spec.links.source is set. Those apps
// build from repoURL; pin tags are main-<sha> and promote shows a changelog.
func (d *Deployable) HasSourceCommits() bool {
	return d != nil && d.Spec.Links.Source && d.Spec.Links.RepoURL != ""
}

// AutoUpdate is the enrolled pin interval, or 0 if the app is track-only.
func (d *Deployable) AutoUpdate() time.Duration {
	if d == nil || d.Spec.Update == nil || d.Spec.Update.Auto == nil {
		return 0
	}
	return d.Spec.Update.Auto.Duration()
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
	if d.Spec.Image.Repository != "" {
		d.Spec.Image.Repository = image.CanonicalRepository(d.Spec.Image.Repository)
	}
	d.Spec.Workload.Kind = cmp.Or(d.Spec.Workload.Kind, "Deployment")
	if d.Spec.Workload.Replicas == 0 {
		d.Spec.Workload.Replicas = 1
	}
	d.Spec.Workload.ContainerName = cmp.Or(d.Spec.Workload.ContainerName, "web")
	if d.Spec.Workload.ContainerPort > 0 {
		d.Spec.Workload.PortName = cmp.Or(d.Spec.Workload.PortName, "http")
	}
	if d.HasRoute() {
		if d.Spec.Route.Port == 0 {
			d.Spec.Route.Port = d.Spec.Workload.ContainerPort
		}
		d.Spec.Route.GatewayNamespace = cmp.Or(d.Spec.Route.GatewayNamespace, "envoy-gateway-system")
		for i := range d.Spec.Stages {
			st := &d.Spec.Stages[i]
			st.Gateway.Group = cmp.Or(st.Gateway.Group, "gateway.networking.k8s.io")
			st.Gateway.Kind = cmp.Or(st.Gateway.Kind, "Gateway")
			st.Gateway.Namespace = cmp.Or(st.Gateway.Namespace, d.Spec.Route.GatewayNamespace)
		}
	}
	d.Spec.Workload.Probes.Port = cmp.Or(d.Spec.Workload.Probes.Port, Port(d.Spec.Workload.PortName))
}

func (d *Deployable) ImageRef() string {
	if d.Spec.Image.Tag == "" {
		return d.Spec.Image.Repository
	}
	return d.Spec.Image.Repository + ":" + d.Spec.Image.Tag
}
