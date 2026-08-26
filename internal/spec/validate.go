package spec

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

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
	if len(d.Spec.Summary) > MaxSummaryLen {
		errs = append(errs, fmt.Sprintf("spec.summary must be at most %d characters", MaxSummaryLen))
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
	if err := validateUpdate(d.Spec.Update); err != "" {
		errs = append(errs, err)
	}
	if err := httpURLError("spec.links.repoURL", d.Spec.Links.RepoURL); err != "" {
		errs = append(errs, err)
	}
	if d.Spec.Links.Source && d.Spec.Links.RepoURL == "" {
		errs = append(errs, "spec.links.repoURL is required when spec.links.source is set")
	}
	if err := httpURLError("spec.links.projectURL", d.Spec.Links.ProjectURL); err != "" {
		errs = append(errs, err)
	}
	if err := httpURLError("spec.links.docsURL", d.Spec.Links.DocsURL); err != "" {
		errs = append(errs, err)
	}
	if err := httpURLError("spec.links.icon", d.Spec.Links.Icon); err != "" {
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

func validateUpdate(u *UpdatePolicy) string {
	if u == nil {
		return ""
	}
	if s := strings.TrimSpace(u.Match); s != "" {
		if _, err := regexp.Compile(s); err != nil {
			return fmt.Sprintf("spec.update.match is not a valid regex: %v", err)
		}
	}
	if u.Auto != nil && u.Auto.Duration() < MinAutoUpdate.Duration() {
		return "spec.update.auto must be at least 1h"
	}
	return ""
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
