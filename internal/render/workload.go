package render

import (
	"path"
	"strconv"

	"github.com/ianunruh/deploybot/internal/spec"
	"github.com/ianunruh/deploybot/internal/yamlx"
)

func writeWorkload(out Tree, d *spec.Deployable) error {
	base := d.Spec.Git.WorkloadPath
	dep, err := yamlx.MarshalGenerated(deploymentObj(d))
	if err != nil {
		return err
	}
	file := workloadFile(d.Spec.Workload.Kind)
	resources := []string{file}
	out[path.Join(base, "base", file)] = dep
	if d.HasRoute() {
		svc, err := yamlx.MarshalGenerated(serviceObj(d))
		if err != nil {
			return err
		}
		rt, err := yamlx.MarshalGenerated(httpRouteObj(d, d.BaseStage()))
		if err != nil {
			return err
		}
		out[path.Join(base, "base/service.yaml")] = svc
		out[path.Join(base, "base/httproute.yaml")] = rt
		resources = append(resources, "service.yaml", "httproute.yaml")
	}
	kust, err := yamlx.MarshalGenerated(kustomization{
		APIVersion: "kustomize.config.k8s.io/v1beta1",
		Kind:       "Kustomization",
		Resources:  resources,
		Labels: []kustomizeLabel{{
			IncludeSelectors: true,
			Pairs: map[string]string{
				labelName:      d.Metadata.Name,
				labelManagedBy: managedBy,
			},
		}},
	})
	if err != nil {
		return err
	}
	out[path.Join(base, "base/kustomization.yaml")] = kust

	for i, st := range d.Spec.Stages {
		overlay := path.Join(base, "overlays", st.Name)
		res := []string{"../../base"}
		var patches []kustomizePatch
		if d.HasRoute() && i > 0 {
			ops := routePatches(d.BaseStage(), st)
			b, err := yamlx.MarshalGenerated(ops)
			if err != nil {
				return err
			}
			out[path.Join(overlay, "patch-httproute.yaml")] = b
			patches = append(patches, kustomizePatch{
				Path: "patch-httproute.yaml",
				Target: &kustomizeTarget{
					Group:   "gateway.networking.k8s.io",
					Version: "v1",
					Kind:    "HTTPRoute",
					Name:    d.Metadata.Name,
				},
			})
		}
		okust, err := yamlx.MarshalGenerated(kustomization{
			APIVersion: "kustomize.config.k8s.io/v1beta1",
			Kind:       "Kustomization",
			Resources:  res,
			Patches:    patches,
		})
		if err != nil {
			return err
		}
		out[path.Join(overlay, "kustomization.yaml")] = okust
	}
	return nil
}

func deploymentObj(d *spec.Deployable) deployment {
	w := d.Spec.Workload
	c := container{
		Name:            w.ContainerName,
		Image:           d.ImageRef(),
		ImagePullPolicy: "IfNotPresent",
	}
	if w.ContainerPort > 0 {
		c.Ports = []containerPort{{
			Name:          w.PortName,
			ContainerPort: w.ContainerPort,
		}}
	}
	probePort := string(w.Probes.Port)
	c.StartupProbe = buildProbe(w.Probes.Path, probePort, w.Probes.Startup)
	c.LivenessProbe = buildProbe(w.Probes.Path, probePort, w.Probes.Liveness)
	c.ReadinessProbe = buildProbe(w.Probes.Path, probePort, w.Probes.Readiness)
	if req := resourceMap(w.Resources.Requests); len(req) > 0 || len(resourceMap(w.Resources.Limits)) > 0 {
		c.Resources = &resources{
			Requests: resourceMap(w.Resources.Requests),
			Limits:   resourceMap(w.Resources.Limits),
		}
	}

	var pullSecrets []localObjectRef
	for _, s := range d.Spec.Image.PullSecrets {
		pullSecrets = append(pullSecrets, localObjectRef{Name: s})
	}

	lbl := labels(d)
	return deployment{
		APIVersion: "apps/v1",
		Kind:       w.Kind,
		Metadata:   objectMeta{Name: d.Metadata.Name, Labels: lbl},
		Spec: deploySpec{
			Replicas: w.Replicas,
			Selector: &labelSel{MatchLabels: map[string]string{labelName: d.Metadata.Name}},
			Template: podTemplate{
				Metadata: objectMeta{Labels: lbl},
				Spec: podSpec{
					ServiceAccountName: w.ServiceAccountName,
					ImagePullSecrets:   pullSecrets,
					Containers:         []container{c},
				},
			},
		},
	}
}

func buildProbe(path, port string, override spec.HTTPProbe) *probe {
	pth := cmpOr(override.Path, path)
	if pth == "" {
		return nil
	}
	return &probe{
		HTTPGet: httpGet{
			Path: pth,
			Port: yamlProbePort(cmpOr(string(override.Port), port)),
		},
		InitialDelaySeconds: override.InitialDelaySeconds,
		PeriodSeconds:       override.PeriodSeconds,
		FailureThreshold:    override.FailureThreshold,
	}
}

// yamlProbePort emits numeric ports as ints so Kubernetes IntOrString
// validation treats them as port numbers, not named ports.
func yamlProbePort(port string) any {
	if n, err := strconv.Atoi(port); err == nil {
		return n
	}
	return port
}

func resourceMap(r spec.ResourceList) map[string]string {
	m := map[string]string{}
	if r.CPU != "" {
		m["cpu"] = r.CPU
	}
	if r.Memory != "" {
		m["memory"] = r.Memory
	}
	if len(m) == 0 {
		return nil
	}
	return m
}

func workloadFile(kind string) string {
	if kind == "StatefulSet" {
		return "statefulset.yaml"
	}
	return "deployment.yaml"
}

func serviceObj(d *spec.Deployable) service {
	return service{
		APIVersion: "v1",
		Kind:       "Service",
		Metadata:   objectMeta{Name: d.Metadata.Name, Labels: labels(d)},
		Spec: serviceSpec{
			Type: "ClusterIP",
			Ports: []servicePort{{
				Name: d.Spec.Workload.PortName,
				Port: d.Spec.Route.Port,
			}},
		},
	}
}

func httpRouteObj(d *spec.Deployable, st spec.Stage) httpRoute {
	rule := httpRule{
		Matches: []httpMatch{{Path: httpPath{Type: "PathPrefix", Value: "/"}}},
		BackendRefs: []backendRef{{
			Name: d.Metadata.Name,
			Port: d.Spec.Route.Port,
		}},
	}
	if d.Spec.Route.Timeout != "" {
		rule.Timeouts = &timeouts{Request: d.Spec.Route.Timeout}
	}
	return httpRoute{
		APIVersion: "gateway.networking.k8s.io/v1",
		Kind:       "HTTPRoute",
		Metadata:   objectMeta{Name: d.Metadata.Name, Labels: labels(d)},
		Spec: httpRouteSpec{
			ParentRefs: []parentRef{{
				Group:       st.Gateway.Group,
				Kind:        st.Gateway.Kind,
				Name:        st.Gateway.Name,
				Namespace:   st.Gateway.Namespace,
				SectionName: st.Gateway.SectionName,
			}},
			Hostnames: []string{st.Hostname},
			Rules:     []httpRule{rule},
		},
	}
}

func routePatches(base, st spec.Stage) []jsonPatchOp {
	var ops []jsonPatchOp
	if st.Gateway.Name != base.Gateway.Name {
		ops = append(ops, jsonPatchOp{Op: "replace", Path: "/spec/parentRefs/0/name", Value: st.Gateway.Name})
	}
	if st.Gateway.SectionName != base.Gateway.SectionName {
		ops = append(ops, jsonPatchOp{Op: "replace", Path: "/spec/parentRefs/0/sectionName", Value: st.Gateway.SectionName})
	}
	if st.Gateway.Namespace != base.Gateway.Namespace {
		ops = append(ops, jsonPatchOp{Op: "replace", Path: "/spec/parentRefs/0/namespace", Value: st.Gateway.Namespace})
	}
	if st.Hostname != base.Hostname {
		ops = append(ops, jsonPatchOp{Op: "replace", Path: "/spec/hostnames/0", Value: st.Hostname})
	}
	return ops
}
