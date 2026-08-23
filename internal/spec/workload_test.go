package spec

import (
	"strings"
	"testing"
)

func TestRejectsUnknownWorkloadKind(t *testing.T) {
	t.Parallel()
	body := strings.ReplaceAll(kmcYAML, "containerPort: 3000", "kind: DaemonSet\n    containerPort: 3000")
	if _, err := Parse([]byte(body)); err == nil {
		t.Fatal("expected error")
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
