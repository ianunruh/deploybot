package render

// Kubernetes / kustomize / Argo objects we emit. Kept as local structs so
// goldens stay stable without pulling the full k8s + Argo type graphs.

type objectMeta struct {
	Name        string            `yaml:"name,omitempty"`
	Namespace   string            `yaml:"namespace,omitempty"`
	Labels      map[string]string `yaml:"labels,omitempty"`
	Annotations map[string]string `yaml:"annotations,omitempty"`
}

type deployment struct {
	APIVersion string     `yaml:"apiVersion"`
	Kind       string     `yaml:"kind"`
	Metadata   objectMeta `yaml:"metadata"`
	Spec       deploySpec `yaml:"spec"`
}

type deploySpec struct {
	Replicas int         `yaml:"replicas"`
	Selector *labelSel   `yaml:"selector,omitempty"`
	Template podTemplate `yaml:"template"`
}

type labelSel struct {
	MatchLabels map[string]string `yaml:"matchLabels"`
}

type podTemplate struct {
	Metadata objectMeta `yaml:"metadata"`
	Spec     podSpec    `yaml:"spec"`
}

type podSpec struct {
	ServiceAccountName string           `yaml:"serviceAccountName,omitempty"`
	ImagePullSecrets   []localObjectRef `yaml:"imagePullSecrets,omitempty"`
	Containers         []container      `yaml:"containers"`
}

type localObjectRef struct {
	Name string `yaml:"name"`
}

type container struct {
	Name            string          `yaml:"name"`
	Image           string          `yaml:"image"`
	ImagePullPolicy string          `yaml:"imagePullPolicy,omitempty"`
	Ports           []containerPort `yaml:"ports,omitempty"`
	StartupProbe    *probe          `yaml:"startupProbe,omitempty"`
	LivenessProbe   *probe          `yaml:"livenessProbe,omitempty"`
	ReadinessProbe  *probe          `yaml:"readinessProbe,omitempty"`
	Resources       *resources      `yaml:"resources,omitempty"`
}

type containerPort struct {
	Name          string `yaml:"name,omitempty"`
	ContainerPort int    `yaml:"containerPort"`
}

type probe struct {
	HTTPGet             httpGet `yaml:"httpGet"`
	InitialDelaySeconds int     `yaml:"initialDelaySeconds,omitempty"`
	PeriodSeconds       int     `yaml:"periodSeconds,omitempty"`
	FailureThreshold    int     `yaml:"failureThreshold,omitempty"`
}

type httpGet struct {
	Path string `yaml:"path"`
	Port string `yaml:"port"`
}

type resources struct {
	Requests map[string]string `yaml:"requests,omitempty"`
	Limits   map[string]string `yaml:"limits,omitempty"`
}

type service struct {
	APIVersion string      `yaml:"apiVersion"`
	Kind       string      `yaml:"kind"`
	Metadata   objectMeta  `yaml:"metadata"`
	Spec       serviceSpec `yaml:"spec"`
}

type serviceSpec struct {
	Type  string        `yaml:"type,omitempty"`
	Ports []servicePort `yaml:"ports"`
}

type servicePort struct {
	Name string `yaml:"name,omitempty"`
	Port int    `yaml:"port"`
}

type httpRoute struct {
	APIVersion string        `yaml:"apiVersion"`
	Kind       string        `yaml:"kind"`
	Metadata   objectMeta    `yaml:"metadata"`
	Spec       httpRouteSpec `yaml:"spec"`
}

type httpRouteSpec struct {
	ParentRefs []parentRef `yaml:"parentRefs"`
	Hostnames  []string    `yaml:"hostnames"`
	Rules      []httpRule  `yaml:"rules"`
}

type parentRef struct {
	Group       string `yaml:"group"`
	Kind        string `yaml:"kind"`
	Name        string `yaml:"name"`
	Namespace   string `yaml:"namespace,omitempty"`
	SectionName string `yaml:"sectionName,omitempty"`
}

type httpRule struct {
	Matches     []httpMatch  `yaml:"matches"`
	Timeouts    *timeouts    `yaml:"timeouts,omitempty"`
	BackendRefs []backendRef `yaml:"backendRefs"`
}

type httpMatch struct {
	Path httpPath `yaml:"path"`
}

type httpPath struct {
	Type  string `yaml:"type"`
	Value string `yaml:"value"`
}

type timeouts struct {
	Request string `yaml:"request"`
}

type backendRef struct {
	Name string `yaml:"name"`
	Port int    `yaml:"port"`
}

type jsonPatchOp struct {
	Op    string `yaml:"op"`
	Path  string `yaml:"path"`
	Value any    `yaml:"value"`
}

type kustomization struct {
	APIVersion         string           `yaml:"apiVersion"`
	Kind               string           `yaml:"kind"`
	Namespace          string           `yaml:"namespace,omitempty"`
	Resources          []string         `yaml:"resources,omitempty"`
	Images             []kustomizeImage `yaml:"images,omitempty"`
	Patches            []kustomizePatch `yaml:"patches,omitempty"`
	ConfigMapGenerator []configMapGen   `yaml:"configMapGenerator,omitempty"`
	Labels             []kustomizeLabel `yaml:"labels,omitempty"`
}

type kustomizeImage struct {
	Name    string `yaml:"name"`
	NewName string `yaml:"newName,omitempty"`
	NewTag  string `yaml:"newTag,omitempty"`
	Digest  string `yaml:"digest,omitempty"`
}

type kustomizePatch struct {
	Path   string           `yaml:"path,omitempty"`
	Target *kustomizeTarget `yaml:"target,omitempty"`
}

type kustomizeTarget struct {
	Group   string `yaml:"group,omitempty"`
	Version string `yaml:"version,omitempty"`
	Kind    string `yaml:"kind,omitempty"`
	Name    string `yaml:"name,omitempty"`
}

type configMapGen struct {
	Name     string   `yaml:"name"`
	Behavior string   `yaml:"behavior,omitempty"`
	Envs     []string `yaml:"envs,omitempty"`
	Files    []string `yaml:"files,omitempty"`
}

type kustomizeLabel struct {
	IncludeSelectors bool              `yaml:"includeSelectors,omitempty"`
	Pairs            map[string]string `yaml:"pairs"`
}

type application struct {
	APIVersion string     `yaml:"apiVersion"`
	Kind       string     `yaml:"kind"`
	Metadata   objectMeta `yaml:"metadata"`
	Spec       appSpec    `yaml:"spec"`
}

type appSpec struct {
	Project     string      `yaml:"project,omitempty"`
	Source      *appSource  `yaml:"source,omitempty"`
	Destination *appDest    `yaml:"destination,omitempty"`
	SyncPolicy  *syncPolicy `yaml:"syncPolicy,omitempty"`
}

type appSource struct {
	Path           string `yaml:"path,omitempty"`
	RepoURL        string `yaml:"repoURL,omitempty"`
	TargetRevision string `yaml:"targetRevision,omitempty"`
}

type appDest struct {
	Namespace string `yaml:"namespace,omitempty"`
	Server    string `yaml:"server,omitempty"`
}

type syncPolicy struct {
	SyncOptions []string `yaml:"syncOptions,omitempty"`
}
