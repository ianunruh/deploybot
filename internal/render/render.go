package render

import (
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/ianunruh/deploybot/internal/image"
	"github.com/ianunruh/deploybot/internal/spec"
	"github.com/ianunruh/deploybot/internal/yamlx"
)

const (
	labelName      = "app.kubernetes.io/name"
	labelManagedBy = "app.kubernetes.io/managed-by"
	managedBy      = "deploybot"
)

// Tree is a map of repo-relative paths to file contents.
type Tree map[string][]byte

func Render(d *spec.Deployable) (Tree, error) {
	out := make(Tree)
	if err := writeWorkload(out, d); err != nil {
		return nil, err
	}
	if err := writeArgo(out, d); err != nil {
		return nil, err
	}
	return out, nil
}

func writeWorkload(out Tree, d *spec.Deployable) error {
	base := d.Spec.Git.WorkloadPath
	dep, err := yamlx.MarshalGenerated(deploymentObj(d))
	if err != nil {
		return err
	}
	svc, err := yamlx.MarshalGenerated(serviceObj(d))
	if err != nil {
		return err
	}
	rt, err := yamlx.MarshalGenerated(httpRouteObj(d, d.BaseStage()))
	if err != nil {
		return err
	}
	kust, err := yamlx.MarshalGenerated(kustomization{
		APIVersion: "kustomize.config.k8s.io/v1beta1",
		Kind:       "Kustomization",
		Resources:  []string{"deployment.yaml", "service.yaml", "httproute.yaml"},
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
	out[path.Join(base, "base/deployment.yaml")] = dep
	out[path.Join(base, "base/service.yaml")] = svc
	out[path.Join(base, "base/httproute.yaml")] = rt
	out[path.Join(base, "base/kustomization.yaml")] = kust

	for i, st := range d.Spec.Stages {
		overlay := path.Join(base, "overlays", st.Name)
		res := []string{"../../base"}
		var patches []kustomizePatch
		if i > 0 {
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
		if len(st.Volumes) > 0 {
			volYAML, err := yamlx.MarshalGenerated(volumePatch(d, st.Volumes))
			if err != nil {
				return err
			}
			out[path.Join(overlay, "patch-volumes.yaml")] = volYAML
			patches = append(patches, kustomizePatch{Path: "patch-volumes.yaml"})
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

func writeArgo(out Tree, d *spec.Deployable) error {
	appPath := d.Spec.Git.ApplicationPath
	baseApp, err := yamlx.MarshalGenerated(applicationObj(d, d.LastStage()))
	if err != nil {
		return err
	}
	out[path.Join(appPath, "base", d.Metadata.Name+".yaml")] = baseApp

	for _, st := range d.Spec.Stages {
		patchApp, err := yamlx.MarshalGenerated(application{
			APIVersion: "argoproj.io/v1alpha1",
			Kind:       "Application",
			Metadata:   objectMeta{Name: d.Spec.Argo.Name},
			Spec: appSpec{
				Source: &appSource{Path: overlayPath(d, st.Name)},
			},
		})
		if err != nil {
			return err
		}
		dir := path.Join(appPath, "overlays", st.Name)
		out[path.Join(dir, d.Metadata.Name+".yaml")] = patchApp
	}
	return nil
}

func overlayPath(d *spec.Deployable, stage string) string {
	return path.Join(d.Spec.Git.WorkloadPath, "overlays", stage)
}

func OverlayKustomizationPath(d *spec.Deployable, stage string) string {
	return path.Join(overlayPath(d, stage), "kustomization.yaml")
}

func labels(d *spec.Deployable) map[string]string {
	return map[string]string{
		labelName:      d.Metadata.Name,
		labelManagedBy: managedBy,
	}
}

func deploymentObj(d *spec.Deployable) deployment {
	w := d.Spec.Workload
	c := container{
		Name:            w.ContainerName,
		Image:           d.ImageRef(),
		ImagePullPolicy: "IfNotPresent",
		Ports: []containerPort{{
			Name:          w.PortName,
			ContainerPort: w.ContainerPort,
		}},
	}
	for _, name := range w.EnvFrom.ConfigMaps {
		c.EnvFrom = append(c.EnvFrom, envFromSource{ConfigMapRef: &localObjectRef{Name: name}})
	}
	for _, name := range w.EnvFrom.Secrets {
		c.EnvFrom = append(c.EnvFrom, envFromSource{SecretRef: &localObjectRef{Name: name}})
	}
	for _, e := range w.Env {
		c.Env = append(c.Env, envVar{Name: e.Name, Value: e.Value})
	}
	var volumes []podVolume
	for _, v := range w.Volumes {
		c.VolumeMounts = append(c.VolumeMounts, volumeMount{
			Name:      v.Name,
			MountPath: v.MountPath,
			ReadOnly:  v.ReadOnly,
		})
		volumes = append(volumes, podVolumeFromSpec(v))
	}
	probePort := w.Probes.Port
	if w.Probes.Path != "" {
		c.StartupProbe = buildProbe(w.Probes.Path, probePort, w.Probes.Startup)
		c.LivenessProbe = buildProbe(w.Probes.Path, probePort, w.Probes.Liveness)
		c.ReadinessProbe = buildProbe(w.Probes.Path, probePort, w.Probes.Readiness)
	}
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
		Kind:       "Deployment",
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
					Volumes:            volumes,
				},
			},
		},
	}
}

func podVolumeFromSpec(v spec.Volume) podVolume {
	pv := podVolume{Name: v.Name}
	switch {
	case v.ConfigMap != "":
		pv.ConfigMap = &configMapVol{Name: v.ConfigMap}
	case v.Secret != "":
		pv.Secret = &secretVol{SecretName: v.Secret, Optional: v.Optional}
	}
	return pv
}

func buildProbe(path, port string, override spec.HTTPProbe) *probe {
	p := &probe{
		HTTPGet: httpGet{
			Path: cmpOr(override.Path, path),
			Port: cmpOr(override.Port, port),
		},
		InitialDelaySeconds: override.InitialDelaySeconds,
		PeriodSeconds:       override.PeriodSeconds,
		FailureThreshold:    override.FailureThreshold,
	}
	return p
}

func cmpOr[T comparable](v, fallback T) T {
	var zero T
	if v == zero {
		return fallback
	}
	return v
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

func volumePatch(d *spec.Deployable, vols []spec.Volume) volumePatchDoc {
	var mounts []volumeMount
	var volumes []podVolume
	for _, v := range vols {
		mounts = append(mounts, volumeMount{
			Name:      v.Name,
			MountPath: v.MountPath,
			ReadOnly:  v.ReadOnly,
		})
		volumes = append(volumes, podVolumeFromSpec(v))
	}
	return volumePatchDoc{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
		Metadata:   objectMeta{Name: d.Metadata.Name},
		Spec: volumePatchSpec{
			Template: volumePatchTemplate{
				Spec: volumePatchPod{
					Containers: []volumePatchContainer{{
						Name:         d.Spec.Workload.ContainerName,
						VolumeMounts: mounts,
					}},
					Volumes: volumes,
				},
			},
		},
	}
}

func applicationObj(d *spec.Deployable, st spec.Stage) application {
	ann := map[string]string{
		"argocd.argoproj.io/manifest-generate-paths": "/" + strings.TrimPrefix(d.Spec.Git.WorkloadPath, "/"),
	}
	app := application{
		APIVersion: "argoproj.io/v1alpha1",
		Kind:       "Application",
		Metadata: objectMeta{
			Name:        d.Spec.Argo.Name,
			Labels:      map[string]string{"argocd.argoproj.io/app-type": "kustomize"},
			Annotations: ann,
		},
		Spec: appSpec{
			Project: d.Spec.Argo.Project,
			Source: &appSource{
				Path:           overlayPath(d, st.Name),
				RepoURL:        d.Spec.Git.RepoURL,
				TargetRevision: d.Spec.Git.TargetRevision,
			},
			Destination: &appDest{
				Namespace: d.Spec.Namespace,
				Server:    d.Spec.Argo.DestinationServer,
			},
		},
	}
	if d.Spec.Argo.CreateNamespace {
		app.Spec.SyncPolicy = &syncPolicy{SyncOptions: []string{"CreateNamespace=true"}}
	}
	return app
}

func upsertImage(images []kustomizeImage, img kustomizeImage) []kustomizeImage {
	for i, existing := range images {
		if existing.Name == img.Name {
			images[i] = img
			return images
		}
	}
	return append(images, img)
}

func CurrentImage(tree Tree, d *spec.Deployable, stage string) (image.Ref, error) {
	p := OverlayKustomizationPath(d, stage)
	b, ok := tree[p]
	if !ok {
		return image.Ref{}, fmt.Errorf("no overlay kustomization for stage %s", stage)
	}
	var k kustomization
	if err := yamlx.Unmarshal(b, &k); err != nil {
		return image.Ref{}, err
	}
	for _, img := range k.Images {
		if img.Name == d.Spec.Image.Repository || img.Name == "" {
			repo := d.Spec.Image.Repository
			if img.NewName != "" {
				repo = img.NewName
			}
			return image.Ref{Repository: repo, Tag: img.NewTag, Digest: img.Digest}, nil
		}
	}
	return image.Ref{}, fmt.Errorf("stage %s has no pinned image", stage)
}

// MergeTrees keeps extra files from existing and merges kustomization.yaml
// files so human-owned generators/patches survive a pin.
func MergeTrees(existing, generated Tree) (Tree, error) {
	out := make(Tree, len(existing)+len(generated))
	for p, b := range existing {
		out[p] = slices.Clone(b)
	}
	for p, gen := range generated {
		cur, ok := out[p]
		if !ok || !strings.HasSuffix(p, "/kustomization.yaml") && p != "kustomization.yaml" {
			out[p] = gen
			continue
		}
		merged, err := mergeKustomization(cur, gen)
		if err != nil {
			return nil, fmt.Errorf("merge %s: %w", p, err)
		}
		out[p] = merged
	}
	return out, nil
}

func mergeKustomization(existing, generated []byte) ([]byte, error) {
	var e, g kustomization
	if len(existing) > 0 {
		if err := yamlx.Unmarshal(existing, &e); err != nil {
			return nil, err
		}
	}
	if err := yamlx.Unmarshal(generated, &g); err != nil {
		return nil, err
	}
	out := e
	out.APIVersion = cmpOr(out.APIVersion, g.APIVersion)
	out.Kind = cmpOr(out.Kind, g.Kind)
	if g.Namespace != "" {
		out.Namespace = g.Namespace
	}
	out.Resources = unionStable(out.Resources, g.Resources)
	for _, img := range g.Images {
		out.Images = upsertImage(out.Images, img)
	}
	out.Patches = unionPatches(out.Patches, g.Patches)
	if len(g.Labels) > 0 && len(out.Labels) == 0 {
		out.Labels = g.Labels
	}
	return yamlx.MarshalGenerated(out)
}

func unionStable(a, b []string) []string {
	seen := make(map[string]struct{}, len(a)+len(b))
	var out []string
	for _, s := range append(slices.Clone(a), b...) {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func unionPatches(a, b []kustomizePatch) []kustomizePatch {
	seen := make(map[string]struct{}, len(a)+len(b))
	var out []kustomizePatch
	for _, p := range append(slices.Clone(a), b...) {
		key := p.Path
		if p.Target != nil {
			key += "|" + p.Target.Kind + "|" + p.Target.Name
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, p)
	}
	return out
}

func SortedPaths(t Tree) []string {
	paths := make([]string, 0, len(t))
	for p := range t {
		paths = append(paths, p)
	}
	slices.Sort(paths)
	return paths
}
