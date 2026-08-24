package kube

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const labelName = "app.kubernetes.io/name"

// Workload is a live Deployment or StatefulSet plus its pods.
type Workload struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Desired   int    `json:"desired"`
	Ready     int    `json:"ready"`
	Updated   int    `json:"updated,omitempty"`
	Available int    `json:"available,omitempty"`
	Message   string `json:"message,omitempty"`
	Pods      []Pod  `json:"pods,omitempty"`
}

// Pod is the kubectl get pod -o wide subset: ready, status, restarts, IP, node.
type Pod struct {
	Name        string     `json:"name"`
	Ready       string     `json:"ready"`
	Status      string     `json:"status"`
	Restarts    int        `json:"restarts"`
	IP          string     `json:"ip,omitempty"`
	Node        string     `json:"node,omitempty"`
	CreatedAt   *time.Time `json:"createdAt,omitempty"`
	RestartedAt *time.Time `json:"restartedAt,omitempty"`
}

type appsWorkload struct {
	Kind     string `json:"kind"`
	Metadata struct {
		Name string `json:"name"`
	} `json:"metadata"`
	Spec struct {
		Replicas *int `json:"replicas"`
		Selector struct {
			MatchLabels map[string]string `json:"matchLabels"`
		} `json:"selector"`
	} `json:"spec"`
	Status struct {
		Replicas          int `json:"replicas"`
		ReadyReplicas     int `json:"readyReplicas"`
		UpdatedReplicas   int `json:"updatedReplicas"`
		AvailableReplicas int `json:"availableReplicas"`
		CurrentReplicas   int `json:"currentReplicas"`
	} `json:"status"`
}

type podList struct {
	Items []pod `json:"items"`
}

type pod struct {
	Metadata struct {
		Name              string     `json:"name"`
		CreationTimestamp *time.Time `json:"creationTimestamp"`
		DeletionTimestamp *time.Time `json:"deletionTimestamp"`
		OwnerReferences   []ownerRef `json:"ownerReferences"`
	} `json:"metadata"`
	Spec struct {
		NodeName       string          `json:"nodeName"`
		InitContainers []namedResource `json:"initContainers"`
		Containers     []namedResource `json:"containers"`
	} `json:"spec"`
	Status struct {
		Phase                 string            `json:"phase"`
		Reason                string            `json:"reason"`
		PodIP                 string            `json:"podIP"`
		InitContainerStatuses []containerStatus `json:"initContainerStatuses"`
		ContainerStatuses     []containerStatus `json:"containerStatuses"`
		Conditions            []podCondition    `json:"conditions"`
	} `json:"status"`
}

type ownerRef struct {
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	Controller *bool  `json:"controller"`
}

type namedResource struct {
	Name string `json:"name"`
}

type containerStatus struct {
	Name         string         `json:"name"`
	Ready        bool           `json:"ready"`
	RestartCount int            `json:"restartCount"`
	State        containerState `json:"state"`
	LastState    containerState `json:"lastState"`
}

type containerState struct {
	Waiting    *stateWaiting    `json:"waiting"`
	Running    *stateRunning    `json:"running"`
	Terminated *stateTerminated `json:"terminated"`
}

type stateWaiting struct {
	Reason string `json:"reason"`
}

type stateRunning struct {
	StartedAt *time.Time `json:"startedAt"`
}

type stateTerminated struct {
	ExitCode   int        `json:"exitCode"`
	Signal     int        `json:"signal"`
	Reason     string     `json:"reason"`
	FinishedAt *time.Time `json:"finishedAt"`
}

type podCondition struct {
	Type   string `json:"type"`
	Status string `json:"status"`
}

// ReadWorkload GETs the Deployment or StatefulSet and lists pods matching its
// selector that are owned by it (or by a ReplicaSet it owns).
func ReadWorkload(ctx context.Context, c *REST, namespace, kind, name string) Workload {
	out := Workload{Kind: defaultKind(kind), Name: name}
	if c == nil || namespace == "" || name == "" {
		out.Message = "no cluster client"
		return out
	}
	var raw appsWorkload
	err := c.Get(ctx, appsPath(namespace, out.Kind, name), &raw)
	selector := map[string]string{labelName: name}
	if err != nil {
		out.Message = workloadErr(out.Kind, name, err)
		if !isNotFound(err) {
			return out
		}
	} else {
		if raw.Kind != "" {
			out.Kind = raw.Kind
		}
		if raw.Metadata.Name != "" {
			out.Name = raw.Metadata.Name
		}
		out.Desired = 1
		if raw.Spec.Replicas != nil {
			out.Desired = *raw.Spec.Replicas
		}
		out.Ready = raw.Status.ReadyReplicas
		out.Updated = raw.Status.UpdatedReplicas
		out.Available = raw.Status.AvailableReplicas
		if len(raw.Spec.Selector.MatchLabels) > 0 {
			selector = raw.Spec.Selector.MatchLabels
		}
	}
	var list podList
	if err := c.Get(ctx, podsPath(namespace, selector), &list); err != nil {
		if out.Message == "" {
			out.Message = fmt.Sprintf("list pods: %s", err.Error())
		}
		return out
	}
	out.Pods = make([]Pod, 0, len(list.Items))
	for _, item := range list.Items {
		if !ownedByWorkload(item, out.Kind, out.Name) {
			continue
		}
		out.Pods = append(out.Pods, summarizePod(item))
	}
	sort.Slice(out.Pods, func(i, j int) bool { return out.Pods[i].Name < out.Pods[j].Name })
	return out
}

func defaultKind(kind string) string {
	if strings.EqualFold(strings.TrimSpace(kind), "StatefulSet") {
		return "StatefulSet"
	}
	return "Deployment"
}

func appsResource(kind string) string {
	if defaultKind(kind) == "StatefulSet" {
		return "statefulsets"
	}
	return "deployments"
}

func appsPath(namespace, kind, name string) string {
	return "/apis/apps/v1/namespaces/" + url.PathEscape(namespace) + "/" + appsResource(kind) + "/" + url.PathEscape(name)
}

func podsPath(namespace string, labels map[string]string) string {
	q := url.Values{}
	q.Set("labelSelector", labelSelector(labels))
	return "/api/v1/namespaces/" + url.PathEscape(namespace) + "/pods?" + q.Encode()
}

func labelSelector(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+labels[k])
	}
	return strings.Join(parts, ",")
}

func isNotFound(err error) bool {
	var se *StatusError
	return errors.As(err, &se) && se.Code == http.StatusNotFound
}

func workloadErr(kind, name string, err error) string {
	if isNotFound(err) {
		return kind + "/" + name + " not found"
	}
	return err.Error()
}

func ownedByWorkload(p pod, kind, name string) bool {
	for _, o := range p.Metadata.OwnerReferences {
		if o.Name == "" || o.Kind == "" {
			continue
		}
		if strings.EqualFold(o.Kind, kind) && o.Name == name {
			return true
		}
		if strings.EqualFold(kind, "Deployment") && strings.EqualFold(o.Kind, "ReplicaSet") && strings.HasPrefix(o.Name, name+"-") {
			return true
		}
	}
	return false
}

func summarizePod(p pod) Pod {
	created := p.Metadata.CreationTimestamp
	if created != nil {
		t := created.UTC()
		created = &t
	}
	restarts, restartedAt := podRestarts(p)
	return Pod{
		Name:        p.Metadata.Name,
		Ready:       podReady(p),
		Status:      podStatus(p),
		Restarts:    restarts,
		IP:          p.Status.PodIP,
		Node:        p.Spec.NodeName,
		CreatedAt:   created,
		RestartedAt: restartedAt,
	}
}

func podReady(p pod) string {
	total := len(p.Spec.Containers)
	if total == 0 {
		total = len(p.Status.ContainerStatuses)
	}
	ready := 0
	for _, cs := range p.Status.ContainerStatuses {
		if cs.Ready && cs.State.Running != nil {
			ready++
		}
	}
	return fmt.Sprintf("%d/%d", ready, total)
}

func podRestarts(p pod) (int, *time.Time) {
	var n int
	var latest *time.Time
	add := func(cs containerStatus) {
		n += cs.RestartCount
		if cs.RestartCount == 0 || cs.LastState.Terminated == nil || cs.LastState.Terminated.FinishedAt == nil {
			return
		}
		t := cs.LastState.Terminated.FinishedAt.UTC()
		if latest == nil || t.After(*latest) {
			latest = &t
		}
	}
	for _, cs := range p.Status.InitContainerStatuses {
		add(cs)
	}
	for _, cs := range p.Status.ContainerStatuses {
		add(cs)
	}
	return n, latest
}

// podStatus mirrors kubectl's STATUS column (phase, waiting reason, Terminating).
func podStatus(p pod) string {
	reason := p.Status.Phase
	if p.Status.Reason != "" {
		reason = p.Status.Reason
	}
	initializing := false
	for i, cs := range p.Status.InitContainerStatuses {
		switch {
		case cs.State.Terminated != nil && cs.State.Terminated.ExitCode == 0:
			continue
		case cs.State.Terminated != nil:
			if cs.State.Terminated.Reason == "" {
				if cs.State.Terminated.Signal != 0 {
					reason = fmt.Sprintf("Init:Signal:%d", cs.State.Terminated.Signal)
				} else {
					reason = fmt.Sprintf("Init:ExitCode:%d", cs.State.Terminated.ExitCode)
				}
			} else {
				reason = "Init:" + cs.State.Terminated.Reason
			}
			initializing = true
		case cs.State.Waiting != nil && cs.State.Waiting.Reason != "" && cs.State.Waiting.Reason != "PodInitializing":
			reason = "Init:" + cs.State.Waiting.Reason
			initializing = true
		default:
			reason = fmt.Sprintf("Init:%d/%d", i, len(p.Spec.InitContainers))
			initializing = true
		}
		break
	}
	if !initializing {
		hasRunning := false
		for i := len(p.Status.ContainerStatuses) - 1; i >= 0; i-- {
			cs := p.Status.ContainerStatuses[i]
			switch {
			case cs.State.Waiting != nil && cs.State.Waiting.Reason != "":
				reason = cs.State.Waiting.Reason
			case cs.State.Terminated != nil && cs.State.Terminated.Reason != "":
				reason = cs.State.Terminated.Reason
			case cs.State.Terminated != nil && cs.State.Terminated.Reason == "":
				if cs.State.Terminated.Signal != 0 {
					reason = fmt.Sprintf("Signal:%d", cs.State.Terminated.Signal)
				} else {
					reason = fmt.Sprintf("ExitCode:%d", cs.State.Terminated.ExitCode)
				}
			case cs.Ready && cs.State.Running != nil:
				hasRunning = true
			}
		}
		if reason == "Completed" && hasRunning {
			if hasReadyCondition(p) {
				reason = "Running"
			} else {
				reason = "NotReady"
			}
		}
	}
	if p.Metadata.DeletionTimestamp != nil {
		if p.Status.Reason == "NodeLost" {
			reason = "Unknown"
		} else {
			reason = "Terminating"
		}
	}
	if reason == "" {
		return "Unknown"
	}
	return reason
}

func hasReadyCondition(p pod) bool {
	for _, c := range p.Status.Conditions {
		if c.Type == "Ready" && c.Status == "True" {
			return true
		}
	}
	return false
}
