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
	if d.Spec.Links.Source || d.HasSourceCommits() {
		t.Fatal("source should default off")
	}
}

func TestParseSourceLinks(t *testing.T) {
	t.Parallel()
	body := kmcYAML + `
  links:
    repoURL: https://github.com/ianunruh/kmc
    source: true
`
	d, err := Parse([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if !d.Spec.Links.Source || !d.HasSourceCommits() {
		t.Fatalf("source %+v", d.Spec.Links)
	}
}

func TestParseCatalogMetadata(t *testing.T) {
	t.Parallel()
	body := kmcYAML + `
  group: platform
  summary: Kubernetes multi-cluster console
  links:
    repoURL: https://github.com/ianunruh/kmc
    docsURL: https://github.com/ianunruh/kmc#readme
    icon: https://github.com/ianunruh/kmc/raw/main/web/public/favicon.svg
`
	d, err := Parse([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if d.Spec.Group != "platform" {
		t.Fatalf("group %q", d.Spec.Group)
	}
	if d.Spec.Summary != "Kubernetes multi-cluster console" {
		t.Fatalf("summary %q", d.Spec.Summary)
	}
	if d.Spec.Links.DocsURL != "https://github.com/ianunruh/kmc#readme" {
		t.Fatalf("docs %q", d.Spec.Links.DocsURL)
	}
	if d.Spec.Links.Icon != "https://github.com/ianunruh/kmc/raw/main/web/public/favicon.svg" {
		t.Fatalf("icon %q", d.Spec.Links.Icon)
	}
}

func TestParseCatalogRejects(t *testing.T) {
	t.Parallel()
	cases := []string{
		kmcYAML + "\n  group: Platform\n",
		kmcYAML + "\n  group: play_media\n",
		kmcYAML + "\n  group: \"-play\"\n",
		kmcYAML + "\n  summary: " + strings.Repeat("x", MaxSummaryLen+1) + "\n",
		kmcYAML + "\n  links:\n    docsURL: javascript:alert(1)\n",
		kmcYAML + "\n  links:\n    icon: \"git@github.com:ianunruh/kmc.git\"\n",
	}
	for _, body := range cases {
		if _, err := Parse([]byte(body)); err == nil {
			t.Fatalf("expected error for %q", body[len(body)-40:])
		}
	}
}

func TestParseSourceRequiresRepoURL(t *testing.T) {
	t.Parallel()
	body := kmcYAML + `
  links:
    source: true
`
	if _, err := Parse([]byte(body)); err == nil {
		t.Fatal("expected error")
	}
}

func TestParseUpdate(t *testing.T) {
	t.Parallel()
	d, err := Parse([]byte(sonarrUpdateYAML("")))
	if err != nil {
		t.Fatal(err)
	}
	if !d.TracksRegistry() {
		t.Fatal("expected tracking")
	}
	if d.AutoUpdate() != 0 {
		t.Fatalf("auto %s", d.AutoUpdate())
	}

	d, err = Parse([]byte(sonarrUpdateYAML("    auto: 24h\n")))
	if err != nil {
		t.Fatal(err)
	}
	if d.AutoUpdate() != 24*time.Hour {
		t.Fatalf("auto %s", d.AutoUpdate())
	}

	d, err = Parse([]byte(sonarrUpdateYAML("    match: '^v?\\d+(\\.\\d+)+-ls\\d+$'\n")))
	if err != nil {
		t.Fatal(err)
	}
	if d.Spec.Update.Match != `^v?\d+(\.\d+)+-ls\d+$` {
		t.Fatalf("match %q", d.Spec.Update.Match)
	}
	re := d.UpdateMatch()
	if re == nil || !re.MatchString("v1.6.0-ls361") || re.MatchString("1.6.0") {
		t.Fatalf("UpdateMatch %v", re)
	}
}

func TestParseUpdateRejectsShortAuto(t *testing.T) {
	t.Parallel()
	for _, auto := range []string{"0", "0s", "30m", "-1h"} {
		if _, err := Parse([]byte(sonarrUpdateYAML("    auto: " + auto + "\n"))); err == nil {
			t.Fatalf("expected error for auto %q", auto)
		}
	}
}

func TestParseUpdateRejectsBadMatch(t *testing.T) {
	t.Parallel()
	if _, err := Parse([]byte(sonarrUpdateYAML("    match: '['\n"))); err == nil {
		t.Fatal("expected error")
	}
}

func sonarrUpdateYAML(autoBlock string) string {
	update := "  update: {}\n"
	if autoBlock != "" {
		update = "  update:\n" + autoBlock
	}
	return `apiVersion: deploybot.kcloud.io/v1alpha1
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
  image:
    repository: docker.io/linuxserver/sonarr
    tag: 4.0.15.2941-ls285
` + update + `  workload:
    kind: StatefulSet
    containerName: sonarr
    containerPort: 8989
  stages:
    - name: homelab
`
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
