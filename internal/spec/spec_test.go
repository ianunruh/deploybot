package spec

import (
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
