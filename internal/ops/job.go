package ops

import (
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	labelManagedBy = "app.kubernetes.io/managed-by"
	labelExecution = "deploybot.io/execution"
	labelKind      = "deploybot.io/kind"
	labelCluster   = "deploybot.io/cluster"
	managedBy      = "deploybot"

	annActorKind = "deploybot.io/actor-kind"
	annActorID   = "deploybot.io/actor-id"
	annRef       = "deploybot.io/ref"
	annDryRun    = "deploybot.io/dry-run"
	annCommand   = "deploybot.io/command"
	annSummary   = "deploybot.io/summary"
	annParams    = "deploybot.io/params"

	generateName = "ops-"
	jobTTL       = int32(14 * 24 * 60 * 60)
	secretsVol   = "ops-ci"
	workdirEnv   = "OPS_WORKDIR"
	repoEnv      = "OPS_REPO_URL"
	refEnv       = "OPS_REF"
)

type objectMeta struct {
	Name              string            `json:"name,omitempty"`
	GenerateName      string            `json:"generateName,omitempty"`
	Namespace         string            `json:"namespace,omitempty"`
	CreationTimestamp *time.Time        `json:"creationTimestamp,omitempty"`
	Labels            map[string]string `json:"labels,omitempty"`
	Annotations       map[string]string `json:"annotations,omitempty"`
}

type batchJob struct {
	APIVersion string         `json:"apiVersion"`
	Kind       string         `json:"kind"`
	Metadata   objectMeta     `json:"metadata"`
	Spec       batchJobSpec   `json:"spec"`
	Status     batchJobStatus `json:"status,omitempty"`
}

type batchJobSpec struct {
	BackoffLimit            *int32          `json:"backoffLimit"`
	TTLSecondsAfterFinished *int32          `json:"ttlSecondsAfterFinished,omitempty"`
	ActiveDeadlineSeconds   *int64          `json:"activeDeadlineSeconds,omitempty"`
	Template                podTemplateSpec `json:"template"`
}

type batchJobStatus struct {
	Active         int            `json:"active,omitempty"`
	Succeeded      int            `json:"succeeded,omitempty"`
	Failed         int            `json:"failed,omitempty"`
	StartTime      *time.Time     `json:"startTime,omitempty"`
	CompletionTime *time.Time     `json:"completionTime,omitempty"`
	Conditions     []jobCondition `json:"conditions,omitempty"`
}

type jobCondition struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type podTemplateSpec struct {
	Metadata objectMeta `json:"metadata"`
	Spec     podSpec    `json:"spec"`
}

type podSpec struct {
	RestartPolicy    string              `json:"restartPolicy"`
	ServiceAccount   string              `json:"serviceAccountName,omitempty"`
	ImagePullSecrets []localObjectRef    `json:"imagePullSecrets,omitempty"`
	Containers       []container         `json:"containers"`
	Volumes          []volume            `json:"volumes,omitempty"`
	Affinity         *affinity           `json:"affinity,omitempty"`
	SecurityContext  *podSecurityContext `json:"securityContext,omitempty"`
}

type localObjectRef struct {
	Name string `json:"name"`
}

type container struct {
	Name            string             `json:"name"`
	Image           string             `json:"image"`
	Command         []string           `json:"command,omitempty"`
	Args            []string           `json:"args,omitempty"`
	Env             []envVar           `json:"env,omitempty"`
	VolumeMounts    []volumeMount      `json:"volumeMounts,omitempty"`
	SecurityContext *containerSecurity `json:"securityContext,omitempty"`
	ImagePullPolicy string             `json:"imagePullPolicy,omitempty"`
}

type envVar struct {
	Name  string `json:"name"`
	Value string `json:"value,omitempty"`
}

type volumeMount struct {
	Name      string `json:"name"`
	MountPath string `json:"mountPath"`
	ReadOnly  bool   `json:"readOnly,omitempty"`
}

type volume struct {
	Name   string        `json:"name"`
	Secret *secretVolume `json:"secret,omitempty"`
}

type secretVolume struct {
	SecretName string `json:"secretName"`
}

type podSecurityContext struct {
	RunAsNonRoot bool   `json:"runAsNonRoot"`
	RunAsUser    *int64 `json:"runAsUser,omitempty"`
	RunAsGroup   *int64 `json:"runAsGroup,omitempty"`
}

type containerSecurity struct {
	AllowPrivilegeEscalation bool         `json:"allowPrivilegeEscalation"`
	RunAsNonRoot             bool         `json:"runAsNonRoot"`
	RunAsUser                *int64       `json:"runAsUser,omitempty"`
	RunAsGroup               *int64       `json:"runAsGroup,omitempty"`
	Capabilities             capabilities `json:"capabilities"`
}

type capabilities struct {
	Drop []string `json:"drop"`
}

type affinity struct {
	NodeAffinity *nodeAffinity `json:"nodeAffinity,omitempty"`
}

type nodeAffinity struct {
	RequiredDuringSchedulingIgnoredDuringExecution *nodeSelector `json:"requiredDuringSchedulingIgnoredDuringExecution,omitempty"`
}

type nodeSelector struct {
	NodeSelectorTerms []nodeSelectorTerm `json:"nodeSelectorTerms"`
}

type nodeSelectorTerm struct {
	MatchExpressions []nodeSelectorRequirement `json:"matchExpressions"`
}

type nodeSelectorRequirement struct {
	Key      string   `json:"key"`
	Operator string   `json:"operator"`
	Values   []string `json:"values,omitempty"`
}

type jobList struct {
	Items []batchJob `json:"items"`
}

type podList struct {
	Items []pod `json:"items"`
}

type pod struct {
	Metadata objectMeta `json:"metadata"`
	Status   podStatus  `json:"status"`
}

type podStatus struct {
	Phase string `json:"phase"`
}

func buildJob(cfg Config, cluster string, k Kind, req Request, argv []string, actor Actor) batchJob {
	zero := int32(0)
	ttl := jobTTL
	deadline := int64(k.Deadline.Seconds())
	sa := k.ServiceAccount
	if sa == "" {
		sa = cfg.sa()
	}
	ref := strings.TrimSpace(req.Ref)
	if ref == "" {
		ref = cfg.ref()
	}
	dry := req.dryRun()
	labels := map[string]string{
		labelManagedBy: managedBy,
		labelExecution: "true",
		labelKind:      k.Name,
		labelCluster:   cluster,
	}
	cmdJSON, _ := json.Marshal(argv)
	ann := map[string]string{
		annRef:     ref,
		annDryRun:  strconv.FormatBool(dry),
		annCommand: string(cmdJSON),
		annSummary: k.Summary(req.Params),
		annParams:  compactJSON(req.Params),
	}
	if actor.Kind != "" {
		ann[annActorKind] = actor.Kind
		ann[annActorID] = actor.ID
	}
	uid := int64(1000)
	job := batchJob{
		APIVersion: "batch/v1",
		Kind:       "Job",
		Metadata: objectMeta{
			GenerateName: generateName,
			Namespace:    cfg.ns(),
			Labels:       labels,
			Annotations:  ann,
		},
		Spec: batchJobSpec{
			BackoffLimit:            &zero,
			TTLSecondsAfterFinished: &ttl,
			ActiveDeadlineSeconds:   &deadline,
			Template: podTemplateSpec{
				Metadata: objectMeta{Labels: labels},
				Spec: podSpec{
					RestartPolicy:    "Never",
					ServiceAccount:   sa,
					ImagePullSecrets: []localObjectRef{{Name: cfg.PullSecret}},
					SecurityContext: &podSecurityContext{
						RunAsNonRoot: true,
						RunAsUser:    &uid,
						RunAsGroup:   &uid,
					},
					Volumes: []volume{{
						Name:   secretsVol,
						Secret: &secretVolume{SecretName: cfg.secret()},
					}},
					Containers: []container{{
						Name:            containerName,
						Image:           cfg.Image,
						ImagePullPolicy: "IfNotPresent",
						Command:         []string{entrypoint},
						Args:            argv,
						Env: []envVar{
							{Name: repoEnv, Value: cfg.RepoURL},
							{Name: refEnv, Value: ref},
							{Name: workdirEnv, Value: k.WorkDir},
						},
						VolumeMounts: []volumeMount{{
							Name:      secretsVol,
							MountPath: secretsMount,
							ReadOnly:  true,
						}},
						SecurityContext: &containerSecurity{
							AllowPrivilegeEscalation: false,
							RunAsNonRoot:             true,
							RunAsUser:                &uid,
							RunAsGroup:               &uid,
							Capabilities:             capabilities{Drop: []string{"ALL"}},
						},
					}},
				},
			},
		},
	}
	if k.AvoidNode != nil {
		if node := strings.TrimSpace(k.AvoidNode(cluster, req.Params)); node != "" {
			job.Spec.Template.Spec.Affinity = avoidNodeAffinity(node)
		}
	}
	if cfg.PullSecret == "" {
		job.Spec.Template.Spec.ImagePullSecrets = nil
	}
	return job
}

func avoidNodeAffinity(node string) *affinity {
	return &affinity{NodeAffinity: &nodeAffinity{
		RequiredDuringSchedulingIgnoredDuringExecution: &nodeSelector{
			NodeSelectorTerms: []nodeSelectorTerm{{
				MatchExpressions: []nodeSelectorRequirement{{
					Key:      "kubernetes.io/hostname",
					Operator: "NotIn",
					Values:   []string{node},
				}},
			}},
		},
	}}
}

func executionSelector(kind, cluster string) string {
	parts := []string{
		labelManagedBy + "=" + managedBy,
		labelExecution + "=true",
	}
	if kind != "" {
		parts = append(parts, labelKind+"="+kind)
	}
	if cluster != "" {
		parts = append(parts, labelCluster+"="+cluster)
	}
	return strings.Join(parts, ",")
}

func jobPhase(j batchJob) string {
	for _, c := range j.Status.Conditions {
		if c.Status != "True" {
			continue
		}
		switch c.Type {
		case "Complete":
			return PhaseSucceeded
		case "Failed":
			return PhaseFailed
		}
	}
	if j.Status.Active > 0 {
		return PhaseRunning
	}
	if j.Status.Succeeded > 0 {
		return PhaseSucceeded
	}
	if j.Status.Failed > 0 {
		return PhaseFailed
	}
	return PhasePending
}

func jobMessage(j batchJob) string {
	for _, c := range j.Status.Conditions {
		if c.Status == "True" && c.Message != "" {
			return c.Message
		}
	}
	return ""
}

func jobActive(j batchJob) bool {
	phase := jobPhase(j)
	return phase == PhasePending || phase == PhaseRunning
}

func executionFromJob(cluster string, j batchJob, podName string) Execution {
	ann := j.Metadata.Annotations
	if ann == nil {
		ann = map[string]string{}
	}
	labels := j.Metadata.Labels
	if labels == nil {
		labels = map[string]string{}
	}
	kind := labels[labelKind]
	if c := labels[labelCluster]; c != "" {
		cluster = c
	}
	ex := Execution{
		ID:        j.Metadata.Name,
		Kind:      kind,
		Cluster:   cluster,
		Phase:     jobPhase(j),
		DryRun:    ann[annDryRun] == "true",
		Ref:       ann[annRef],
		Summary:   ann[annSummary],
		Actor:     Actor{Kind: ann[annActorKind], ID: ann[annActorID]},
		PodName:   podName,
		Message:   jobMessage(j),
		CreatedAt: j.Metadata.CreationTimestamp,
	}
	if raw := strings.TrimSpace(ann[annCommand]); raw != "" {
		var argv []string
		if json.Unmarshal([]byte(raw), &argv) == nil {
			ex.Command = argv
		}
	}
	if raw := strings.TrimSpace(ann[annParams]); raw != "" {
		ex.Params = json.RawMessage(raw)
	}
	return ex
}

func jobsPath(ns string) string {
	return "/apis/batch/v1/namespaces/" + url.PathEscape(ns) + "/jobs"
}

func jobPath(ns, name string) string {
	return jobsPath(ns) + "/" + url.PathEscape(name)
}

func podsPath(ns string) string {
	return "/api/v1/namespaces/" + url.PathEscape(ns) + "/pods"
}

func podLogPath(ns, pod string, follow bool) string {
	q := url.Values{
		"container":  {containerName},
		"timestamps": {"true"},
	}
	if follow {
		q.Set("follow", "true")
	}
	return "/api/v1/namespaces/" + url.PathEscape(ns) + "/pods/" + url.PathEscape(pod) + "/log?" + q.Encode()
}
