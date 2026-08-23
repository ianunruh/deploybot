package spec

import (
	"cmp"
	"fmt"
	"net/url"
	"os"
	"strings"
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
	Namespace string   `yaml:"namespace"`
	Links     Links    `yaml:"links,omitempty"`
	Git       Git      `yaml:"git"`
	Argo      Argo     `yaml:"argo"`
	Image     Image    `yaml:"image"`
	Workload  Workload `yaml:"workload"`
	Route     Route    `yaml:"route"`
	Stages    []Stage  `yaml:"stages"`
}

// Links are optional URLs shown in the console. RepoURL is the app source
// (GitHub/GitLab). ProjectURL is a tracker or board (Trello, Linear, …).
type Links struct {
	RepoURL    string `yaml:"repoURL,omitempty"`
	ProjectURL string `yaml:"projectURL,omitempty"`
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

type Route struct {
	Port             int    `yaml:"port,omitempty"`
	Timeout          string `yaml:"timeout,omitempty"`
	GatewayNamespace string `yaml:"gatewayNamespace,omitempty"`
}

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
	if err := httpURLError("spec.links.repoURL", d.Spec.Links.RepoURL); err != "" {
		errs = append(errs, err)
	}
	if err := httpURLError("spec.links.projectURL", d.Spec.Links.ProjectURL); err != "" {
		errs = append(errs, err)
	}
	if d.Spec.Workload.Kind != "Deployment" && d.Spec.Workload.Kind != "StatefulSet" {
		errs = append(errs, "spec.workload.kind must be Deployment or StatefulSet")
	}
	hasRoute := d.HasRoute()
	if hasRoute && d.Spec.Route.Port <= 0 {
		errs = append(errs, "spec.workload.containerPort or spec.route.port is required when a route is configured")
	}
	if len(d.Spec.Stages) == 0 {
		errs = append(errs, "spec.stages must have at least one stage")
	}
	seen := make(map[string]struct{}, len(d.Spec.Stages))
	for i, st := range d.Spec.Stages {
		if st.Name == "" {
			errs = append(errs, "stage name is required")
			continue
		}
		if _, ok := seen[st.Name]; ok {
			errs = append(errs, fmt.Sprintf("duplicate stage %q", st.Name))
		}
		if err := validatePromote(st, i, seen); err != "" {
			errs = append(errs, err)
		}
		seen[st.Name] = struct{}{}
		if !hasRoute {
			continue
		}
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

func validatePromote(st Stage, index int, earlier map[string]struct{}) string {
	p := st.Promote
	if p == nil {
		return ""
	}
	if index == 0 {
		return fmt.Sprintf("stage %s: promote is not allowed on the first stage", st.Name)
	}
	if len(p.After) == 0 {
		return fmt.Sprintf("stage %s: promote.after is required", st.Name)
	}
	seenGate := map[string]struct{}{}
	for _, g := range p.After {
		if g == "" {
			return fmt.Sprintf("stage %s: promote.after entry is empty", st.Name)
		}
		switch g {
		case AfterHealthy, AfterBake, AfterApproval:
		default:
			return fmt.Sprintf("stage %s: unknown promote.after %q (healthy, bake, approval)", st.Name, g)
		}
		if _, ok := seenGate[g]; ok {
			return fmt.Sprintf("stage %s: duplicate promote.after %q", st.Name, g)
		}
		seenGate[g] = struct{}{}
	}
	if p.Has(AfterBake) && p.Bake.Duration() <= 0 {
		return fmt.Sprintf("stage %s: promote.bake is required when after includes bake", st.Name)
	}
	if p.Bake.Duration() > 0 && !p.Has(AfterBake) {
		return fmt.Sprintf("stage %s: promote.bake is set but after does not include bake", st.Name)
	}
	if p.From == "" {
		return ""
	}
	if p.From == st.Name {
		return fmt.Sprintf("stage %s: promote.from cannot be itself", st.Name)
	}
	if _, ok := earlier[p.From]; !ok {
		return fmt.Sprintf("stage %s: promote.from %q must be an earlier stage", st.Name, p.From)
	}
	return ""
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

// HasRoute is true when the spec describes an HTTPRoute (timeout, port,
// gateway namespace, or any stage hostname/gateway). Controllers omit this.
func (d *Deployable) HasRoute() bool {
	r := d.Spec.Route
	if r.Port != 0 || r.Timeout != "" || r.GatewayNamespace != "" {
		return true
	}
	for _, st := range d.Spec.Stages {
		if st.Hostname != "" || st.Gateway.Name != "" || st.Gateway.SectionName != "" {
			return true
		}
	}
	return false
}

func (d *Deployable) ImageRef() string {
	if d.Spec.Image.Tag == "" {
		return d.Spec.Image.Repository
	}
	return d.Spec.Image.Repository + ":" + d.Spec.Image.Tag
}

func httpURLError(field, raw string) string {
	if raw == "" {
		return ""
	}
	u, err := url.ParseRequestURI(raw)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return fmt.Sprintf("%s must be an http(s) URL", field)
	}
	return ""
}
