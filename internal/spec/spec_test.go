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

const sonarrYAML = `
apiVersion: deploybot.kcloud.io/v1alpha1
kind: Deployable
metadata:
  name: sonarr
spec:
  namespace: play
  git:
    repoURL: https://github.com/ianunruh/kcloud-ops
    workloadPath: k8s/play/sonarr
    applicationPath: k8s/apps/projects/play
  argo:
    project: play
    name: play-sonarr
  image:
    repository: lscr.io/linuxserver/sonarr
    tag: 4.0.15.2941-ls285
  workload:
    kind: StatefulSet
    containerName: sonarr
    containerPort: 8989
  route: {}
  stages:
    - name: homelab
      hostname: sonarr.k8s.kcloud.zone
      gateway:
        name: internal
        sectionName: https
    - name: prod
      hostname: sonarr.kcloud.io
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

func TestParseSonarr(t *testing.T) {
	t.Parallel()
	d, err := Parse([]byte(sonarrYAML))
	if err != nil {
		t.Fatal(err)
	}
	if d.Spec.Workload.Kind != "StatefulSet" {
		t.Fatalf("kind %q", d.Spec.Workload.Kind)
	}
	if d.Spec.Argo.Name != "play-sonarr" {
		t.Fatalf("argo name %q", d.Spec.Argo.Name)
	}
	if d.Spec.Image.Repository != "docker.io/linuxserver/sonarr" {
		t.Fatalf("canonical image %q", d.Spec.Image.Repository)
	}
	if d.Spec.Workload.ContainerName != "sonarr" {
		t.Fatalf("container %q", d.Spec.Workload.ContainerName)
	}
	if !d.HasRoute() {
		t.Fatal("expected route")
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
