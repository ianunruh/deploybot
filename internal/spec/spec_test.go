package spec

import (
	"strings"
	"testing"

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
