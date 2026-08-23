package spec

import (
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

const kmcYAML = `
apiVersion: deploybot.kcloud.io/v1alpha1
kind: Deployable
metadata:
  name: kmc
spec:
  namespace: kmc-system
  git:
    repoURL: https://github.com/ianunruh/kcloud-ops
    workloadPath: k8s/kmc
    applicationPath: k8s/apps/projects/sandbox
  argo:
    project: sandbox
    createNamespace: true
  image:
    repository: ghcr.io/ianunruh/kmc
    tag: main
    pullSecrets:
      - ghcr-auth
  workload:
    containerPort: 3000
    serviceAccountName: kmc
    probes:
      path: /login
  route:
    timeout: 3600s
  stages:
    - name: homelab
      hostname: console.kcloud.zone
      gateway:
        name: internal
        sectionName: https-public
    - name: prod
      hostname: console.kcloud.io
      gateway:
        name: external
        sectionName: https-public
`

func TestParseKMC(t *testing.T) {
	t.Parallel()
	d, err := Parse([]byte(kmcYAML))
	if err != nil {
		t.Fatal(err)
	}
	if d.Metadata.Name != "kmc" {
		t.Fatalf("name %q", d.Metadata.Name)
	}
	if d.Spec.Links.RepoURL != "" || d.Spec.Links.ProjectURL != "" {
		t.Fatalf("links %+v", d.Spec.Links)
	}
	if got := d.StageNames(); !cmp.Equal(got, []string{"homelab", "prod"}) {
		t.Fatalf("stages %v", got)
	}
	if d.Spec.Workload.ContainerName != "web" {
		t.Fatalf("default container %q", d.Spec.Workload.ContainerName)
	}
	if d.Spec.Route.Port != 3000 {
		t.Fatalf("route port %d", d.Spec.Route.Port)
	}
	st, err := d.Stage("prod")
	if err != nil {
		t.Fatal(err)
	}
	if st.Gateway.Namespace != "envoy-gateway-system" {
		t.Fatalf("gateway ns %q", st.Gateway.Namespace)
	}
}

func TestParseRejectsEmpty(t *testing.T) {
	t.Parallel()
	if _, err := Parse([]byte("kind: Deployable\n")); err == nil {
		t.Fatal("expected error")
	}
}

func TestUnknownStage(t *testing.T) {
	t.Parallel()
	d, err := Parse([]byte(kmcYAML))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Stage("gamma"); err == nil {
		t.Fatal("expected error")
	}
}

const controllerYAML = `
apiVersion: deploybot.kcloud.io/v1alpha1
kind: Deployable
metadata:
  name: kmc-controller
spec:
  namespace: kmc-system
  git:
    repoURL: https://github.com/ianunruh/kcloud-ops
    workloadPath: k8s/kmc-controller
    applicationPath: k8s/apps/projects/sandbox
  argo:
    project: sandbox
  image:
    repository: ghcr.io/ianunruh/kmc-controller
    tag: main
  workload:
    containerName: manager
    serviceAccountName: kmc-controller
  stages:
    - name: homelab
    - name: prod
`

func TestParseControllerOmitsRoute(t *testing.T) {
	t.Parallel()
	d, err := Parse([]byte(controllerYAML))
	if err != nil {
		t.Fatal(err)
	}
	if d.HasRoute() {
		t.Fatal("controller spec should not have a route")
	}
	if d.Spec.Workload.ContainerPort != 0 {
		t.Fatalf("containerPort %d", d.Spec.Workload.ContainerPort)
	}
	if d.Spec.Workload.PortName != "" {
		t.Fatalf("portName defaulted to %q", d.Spec.Workload.PortName)
	}
	if d.Spec.Route.GatewayNamespace != "" {
		t.Fatalf("gateway namespace defaulted to %q", d.Spec.Route.GatewayNamespace)
	}
	st, err := d.Stage("homelab")
	if err != nil {
		t.Fatal(err)
	}
	if st.Hostname != "" || st.Gateway.Name != "" {
		t.Fatalf("stage %+v", st)
	}
}

func TestRouteStillRequiresHostname(t *testing.T) {
	t.Parallel()
	body := strings.ReplaceAll(kmcYAML, "hostname: console.kcloud.zone", "hostname: \"\"")
	if _, err := Parse([]byte(body)); err == nil {
		t.Fatal("expected hostname required when a route is configured")
	}
}

func TestRouteRequiresPort(t *testing.T) {
	t.Parallel()
	body := strings.ReplaceAll(kmcYAML, "containerPort: 3000\n", "")
	if _, err := Parse([]byte(body)); err == nil {
		t.Fatal("expected port required when a route is configured")
	}
}

func TestParseLinks(t *testing.T) {
	t.Parallel()
	body := kmcYAML + `
  links:
    repoURL: https://github.com/ianunruh/kmc
    projectURL: https://trello.com/b/abc/kmc
`
	d, err := Parse([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if d.Spec.Links.RepoURL != "https://github.com/ianunruh/kmc" {
		t.Fatalf("repo %q", d.Spec.Links.RepoURL)
	}
	if d.Spec.Links.ProjectURL != "https://trello.com/b/abc/kmc" {
		t.Fatalf("project %q", d.Spec.Links.ProjectURL)
	}
}

func TestLinksMustBeHTTP(t *testing.T) {
	t.Parallel()
	body := kmcYAML + `
  links:
    repoURL: "git@github.com:ianunruh/kmc.git"
`
	if _, err := Parse([]byte(body)); err == nil {
		t.Fatal("expected ssh repo URL rejected")
	}
	body = kmcYAML + `
  links:
    projectURL: "javascript:alert(1)"
`
	if _, err := Parse([]byte(body)); err == nil {
		t.Fatal("expected javascript project URL rejected")
	}
}

func TestProbePortAcceptsIntegerAndName(t *testing.T) {
	t.Parallel()
	const body = `
apiVersion: deploybot.kcloud.io/v1alpha1
kind: Deployable
metadata:
  name: kmc-controller
spec:
  namespace: kmc-system
  git:
    repoURL: https://github.com/ianunruh/kcloud-ops
    workloadPath: k8s/kmc-controller
    applicationPath: k8s/apps/projects/sandbox
  argo:
    project: sandbox
  image:
    repository: ghcr.io/ianunruh/kmc-controller
  workload:
    probes:
      port: 8081
      liveness:
        path: /healthz
        port: "8081"
      readiness:
        path: /readyz
        port: http
  stages:
    - name: homelab
`
	d, err := Parse([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if d.Spec.Workload.Probes.Port != "8081" {
		t.Fatalf("shared port %q", d.Spec.Workload.Probes.Port)
	}
	if d.Spec.Workload.Probes.Liveness.Port != "8081" {
		t.Fatalf("liveness port %q", d.Spec.Workload.Probes.Liveness.Port)
	}
	if d.Spec.Workload.Probes.Readiness.Port != "http" {
		t.Fatalf("readiness port %q", d.Spec.Workload.Probes.Readiness.Port)
	}
}

func TestParsePromotePolicy(t *testing.T) {
	t.Parallel()
	body := kmcYAML + `
      promote:
        after:
          - bake
          - approval
        bake: 30m
`
	d, err := Parse([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	st, err := d.Stage("prod")
	if err != nil {
		t.Fatal(err)
	}
	if st.Promote == nil || !st.Promote.Has(AfterBake) || !st.Promote.Has(AfterApproval) {
		t.Fatalf("promote %+v", st.Promote)
	}
	if st.Promote.Bake.Duration() != 30*time.Minute {
		t.Fatalf("bake %s", st.Promote.Bake.Duration())
	}
	if st.Promote.AutoPromote() {
		t.Fatal("approval should block auto-promote")
	}
	src, err := d.SourceStage("prod")
	if err != nil {
		t.Fatal(err)
	}
	if src != "homelab" {
		t.Fatalf("source %q", src)
	}
}

func TestParsePromoteAfterMustBeList(t *testing.T) {
	t.Parallel()
	body := kmcYAML + `
      promote:
        after: approval
`
	if _, err := Parse([]byte(body)); err == nil {
		t.Fatal("expected scalar after to be rejected")
	}
}

func TestParsePromoteRejectsFirstStage(t *testing.T) {
	t.Parallel()
	body := strings.Replace(kmcYAML, "    - name: homelab\n", `    - name: homelab
      promote:
        after:
          - approval
`, 1)
	if _, err := Parse([]byte(body)); err == nil {
		t.Fatal("expected first-stage promote rejected")
	}
}

func TestParsePromoteBakeRequired(t *testing.T) {
	t.Parallel()
	body := kmcYAML + `
      promote:
        after:
          - bake
`
	if _, err := Parse([]byte(body)); err == nil {
		t.Fatal("expected bake duration required")
	}
}
