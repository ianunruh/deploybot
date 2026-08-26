package release

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ianunruh/deploybot/internal/catalog"
	"github.com/ianunruh/deploybot/internal/gitwrite"
	"github.com/ianunruh/deploybot/internal/image"
	"github.com/ianunruh/deploybot/internal/render"
)

const miniSonarr = `
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
    repository: docker.io/linuxserver/sonarr
    tag: 4.0.15
  update:
    auto: 24h
  workload:
    kind: StatefulSet
    containerName: sonarr
    containerPort: 8989
  stages:
    - name: homelab
    - name: prod
      promote:
        after:
          - approval
`

const miniSonarrMatch = `
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
    repository: docker.io/linuxserver/sonarr
    tag: 4.0.15.2941-ls285
  update:
    auto: 24h
    match: '^v?\d+(\.\d+)+-ls\d+$'
  workload:
    kind: StatefulSet
    containerName: sonarr
    containerPort: 8989
  stages:
    - name: homelab
    - name: prod
      promote:
        after:
          - approval
`

const miniSonarrTrackOnly = `
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
  image:
    repository: docker.io/linuxserver/sonarr
    tag: 4.0.15
  update: {}
  workload:
    kind: StatefulSet
    containerName: sonarr
    containerPort: 8989
  stages:
    - name: homelab
`

type fakeList struct {
	versions []image.Version
	err      error
	calls    int
}

func (f *fakeList) List(_ context.Context, _ string, _ string) (image.Listing, error) {
	f.calls++
	if f.err != nil {
		return image.Listing{}, f.err
	}
	return image.Listing{Source: "dockerhub", Versions: f.versions}, nil
}

func sonarrVer(tag, digest string, age time.Duration) image.Version {
	ref := image.Ref{Repository: "docker.io/linuxserver/sonarr", Tag: tag, Digest: digest}
	return image.Version{
		Repository: ref.Repository,
		Ref:        ref.String(),
		Tag:        tag,
		Digest:     digest,
		Tags:       []string{tag},
		CreatedAt:  time.Now().UTC().Add(-age),
	}
}

func catalogNamed(t *testing.T, name, body string) *catalog.Catalog {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, name+".yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cat, err := catalog.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	return cat
}

func TestCheckUpdateRejectsOwned(t *testing.T) {
	t.Parallel()
	svc := &Service{Catalog: loadExamples(t)}
	if _, err := svc.CheckUpdate(t.Context(), "kmc"); err == nil {
		t.Fatal("expected error")
	}
}

func TestCheckUpdateStale(t *testing.T) {
	t.Parallel()
	dir := initOpsRepo(t)
	lister := &fakeList{versions: []image.Version{
		sonarrVer("4.0.16", "sha256:new", 0),
		sonarrVer("latest", "sha256:new", 0),
	}}
	svc := &Service{
		Catalog: catalogNamed(t, "sonarr", miniSonarr),
		OpsRepo: dir,
		Apply:   true,
		Author:  gitwrite.Author{Name: "t", Email: "t@t"},
		Images:  lister,
	}
	if _, err := svc.Pin(t.Context(), "sonarr", "homelab", "docker.io/linuxserver/sonarr:4.0.15@sha256:old"); err != nil {
		t.Fatal(err)
	}
	st, err := svc.CheckUpdate(t.Context(), "sonarr")
	if err != nil {
		t.Fatal(err)
	}
	if !st.Stale || st.Newest == nil || st.Newest.Tag != "4.0.16" || st.Auto != "24h" {
		t.Fatalf("%+v", st)
	}
}

func TestCheckUpdateMatchSelectsLinuxServerTag(t *testing.T) {
	t.Parallel()
	dir := initOpsRepo(t)
	lister := &fakeList{versions: []image.Version{
		sonarrVer("1.6.0", "sha256:new", 0),
		sonarrVer("version-v1.6.0", "sha256:new", 0),
		sonarrVer("v1.6.0-ls361", "sha256:new", 0),
		sonarrVer("latest", "sha256:new", 0),
		sonarrVer("1.6.1-development", "sha256:dev", time.Minute),
	}}
	svc := &Service{
		Catalog: catalogNamed(t, "sonarr", miniSonarrMatch),
		OpsRepo: dir,
		Apply:   true,
		Author:  gitwrite.Author{Name: "t", Email: "t@t"},
		Images:  lister,
	}
	if _, err := svc.Pin(t.Context(), "sonarr", "homelab", "docker.io/linuxserver/sonarr:1.6.0@sha256:old"); err != nil {
		t.Fatal(err)
	}
	st, err := svc.CheckUpdate(t.Context(), "sonarr")
	if err != nil {
		t.Fatal(err)
	}
	if !st.Stale || st.Newest == nil || st.Newest.Tag != "v1.6.0-ls361" {
		t.Fatalf("%+v", st)
	}
}

func TestCheckUpdateMatchSameDigestNotStale(t *testing.T) {
	t.Parallel()
	dir := initOpsRepo(t)
	lister := &fakeList{versions: []image.Version{
		sonarrVer("1.6.0", "sha256:same", 0),
		sonarrVer("v1.6.0-ls361", "sha256:same", 0),
		sonarrVer("latest", "sha256:same", 0),
	}}
	svc := &Service{
		Catalog: catalogNamed(t, "sonarr", miniSonarrMatch),
		OpsRepo: dir,
		Apply:   true,
		Author:  gitwrite.Author{Name: "t", Email: "t@t"},
		Images:  lister,
	}
	if _, err := svc.Pin(t.Context(), "sonarr", "homelab", "docker.io/linuxserver/sonarr:v1.6.0-ls361@sha256:same"); err != nil {
		t.Fatal(err)
	}
	st, err := svc.CheckUpdate(t.Context(), "sonarr")
	if err != nil {
		t.Fatal(err)
	}
	if st.Stale || st.Newest == nil || st.Newest.Tag != "v1.6.0-ls361" {
		t.Fatalf("%+v", st)
	}
}

func TestCheckUpdateMatchNoTagsErrors(t *testing.T) {
	t.Parallel()
	lister := &fakeList{versions: []image.Version{
		sonarrVer("1.6.0", "sha256:new", 0),
		sonarrVer("latest", "sha256:new", 0),
	}}
	svc := &Service{
		Catalog: catalogNamed(t, "sonarr", miniSonarrMatch),
		Images:  lister,
	}
	st, err := svc.CheckUpdate(t.Context(), "sonarr")
	if err != nil {
		t.Fatal(err)
	}
	if st.Error == "" || st.Newest != nil || st.Stale {
		t.Fatalf("%+v", st)
	}
}

func TestReconcileUpdatesPinsEnrolled(t *testing.T) {
	t.Parallel()
	dir := initOpsRepo(t)
	lister := &fakeList{versions: []image.Version{
		sonarrVer("4.0.16", "sha256:new", 0),
	}}
	svc := &Service{
		Catalog: catalogNamed(t, "sonarr", miniSonarr),
		OpsRepo: dir,
		Apply:   true,
		AutoPin: true,
		Author:  gitwrite.Author{Name: "t", Email: "t@t"},
		Images:  lister,
	}
	if _, err := svc.Pin(t.Context(), "sonarr", "homelab", "docker.io/linuxserver/sonarr:4.0.15@sha256:old"); err != nil {
		t.Fatal(err)
	}
	svc.ReconcileUpdates(t.Context())
	tree := mustOpenTree(t, dir)
	d, err := svc.Catalog.Get("sonarr")
	if err != nil {
		t.Fatal(err)
	}
	img, err := render.CurrentImage(tree, d, "homelab")
	if err != nil {
		t.Fatal(err)
	}
	if img.Digest != "sha256:new" || img.Tag != "4.0.16" {
		t.Fatalf("homelab %+v", img)
	}
	if _, err := render.CurrentImage(tree, d, "prod"); err == nil {
		t.Fatal("must not pin prod")
	}
}

func TestReconcileUpdatesSkipsTrackOnly(t *testing.T) {
	t.Parallel()
	dir := initOpsRepo(t)
	lister := &fakeList{versions: []image.Version{
		sonarrVer("4.0.16", "sha256:new", 0),
	}}
	svc := &Service{
		Catalog: catalogNamed(t, "sonarr", miniSonarrTrackOnly),
		OpsRepo: dir,
		Apply:   true,
		Author:  gitwrite.Author{Name: "t", Email: "t@t"},
		Images:  lister,
	}
	if _, err := svc.Pin(t.Context(), "sonarr", "homelab", "docker.io/linuxserver/sonarr:4.0.15@sha256:old"); err != nil {
		t.Fatal(err)
	}
	svc.ReconcileUpdates(t.Context())
	tree := mustOpenTree(t, dir)
	d, err := svc.Catalog.Get("sonarr")
	if err != nil {
		t.Fatal(err)
	}
	img, err := render.CurrentImage(tree, d, "homelab")
	if err != nil {
		t.Fatal(err)
	}
	if img.Digest != "sha256:old" {
		t.Fatalf("track-only must not auto-pin, got %+v", img)
	}
}

func TestReconcileUpdatesSkipsCurrent(t *testing.T) {
	t.Parallel()
	dir := initOpsRepo(t)
	lister := &fakeList{versions: []image.Version{
		sonarrVer("4.0.15", "sha256:old", 0),
		sonarrVer("latest", "sha256:old", 0),
	}}
	svc := &Service{
		Catalog: catalogNamed(t, "sonarr", miniSonarr),
		OpsRepo: dir,
		Apply:   true,
		AutoPin: true,
		Author:  gitwrite.Author{Name: "t", Email: "t@t"},
		Images:  lister,
	}
	if _, err := svc.Pin(t.Context(), "sonarr", "homelab", "docker.io/linuxserver/sonarr:4.0.15@sha256:old"); err != nil {
		t.Fatal(err)
	}
	head := repoHeadHash(t, dir)
	svc.ReconcileUpdates(t.Context())
	if got := repoHeadHash(t, dir); got != head {
		t.Fatal("current image must not create a pin commit")
	}
}

func TestReconcileUpdatesSkipsWhenNotApply(t *testing.T) {
	t.Parallel()
	dir := initOpsRepo(t)
	lister := &fakeList{versions: []image.Version{
		sonarrVer("4.0.16", "sha256:new", 0),
	}}
	svc := &Service{
		Catalog: catalogNamed(t, "sonarr", miniSonarr),
		OpsRepo: dir,
		Apply:   true,
		AutoPin: true,
		Author:  gitwrite.Author{Name: "t", Email: "t@t"},
		Images:  lister,
	}
	if _, err := svc.Pin(t.Context(), "sonarr", "homelab", "docker.io/linuxserver/sonarr:4.0.15@sha256:old"); err != nil {
		t.Fatal(err)
	}
	svc.Apply = false
	svc.ReconcileUpdates(t.Context())
	tree := mustOpenTree(t, dir)
	d, err := svc.Catalog.Get("sonarr")
	if err != nil {
		t.Fatal(err)
	}
	img, err := render.CurrentImage(tree, d, "homelab")
	if err != nil {
		t.Fatal(err)
	}
	if img.Digest != "sha256:old" {
		t.Fatalf("must not auto-pin without apply, got %+v", img)
	}
}

func TestReconcileUpdatesListingErrorDoesNotPin(t *testing.T) {
	t.Parallel()
	dir := initOpsRepo(t)
	lister := &fakeList{err: errors.New("hub 429")}
	svc := &Service{
		Catalog: catalogNamed(t, "sonarr", miniSonarr),
		OpsRepo: dir,
		Apply:   true,
		AutoPin: true,
		Author:  gitwrite.Author{Name: "t", Email: "t@t"},
		Images:  lister,
	}
	if _, err := svc.Pin(t.Context(), "sonarr", "homelab", "docker.io/linuxserver/sonarr:4.0.15@sha256:old"); err != nil {
		t.Fatal(err)
	}
	head := repoHeadHash(t, dir)
	svc.ReconcileUpdates(t.Context())
	if got := repoHeadHash(t, dir); got != head {
		t.Fatal("listing error must not pin")
	}
}

func TestReconcileUpdatesSkipsWhenNotAutoPin(t *testing.T) {
	t.Parallel()
	dir := initOpsRepo(t)
	lister := &fakeList{versions: []image.Version{
		sonarrVer("4.0.16", "sha256:new", 0),
	}}
	svc := &Service{
		Catalog: catalogNamed(t, "sonarr", miniSonarr),
		OpsRepo: dir,
		Apply:   true,
		Author:  gitwrite.Author{Name: "t", Email: "t@t"},
		Images:  lister,
	}
	if _, err := svc.Pin(t.Context(), "sonarr", "homelab", "docker.io/linuxserver/sonarr:4.0.15@sha256:old"); err != nil {
		t.Fatal(err)
	}
	svc.ReconcileUpdates(t.Context())
	tree := mustOpenTree(t, dir)
	d, err := svc.Catalog.Get("sonarr")
	if err != nil {
		t.Fatal(err)
	}
	img, err := render.CurrentImage(tree, d, "homelab")
	if err != nil {
		t.Fatal(err)
	}
	if img.Digest != "sha256:old" {
		t.Fatalf("must not auto-pin without AutoPin, got %+v", img)
	}
}

func TestListUpdatesOmitsOwned(t *testing.T) {
	t.Parallel()
	lister := &fakeList{versions: []image.Version{
		sonarrVer("4.0.16.2945-ls286", "sha256:new", 0),
	}}
	svc := &Service{Catalog: loadExamples(t), Images: lister}
	svc.RefreshUpdates(t.Context())
	got := svc.ListUpdates(t.Context())
	names := make([]string, 0, len(got.Updates))
	for _, u := range got.Updates {
		names = append(names, u.Name)
	}
	for _, n := range []string{"kmc", "kmc-controller", "deploybot", "deploybot-web"} {
		for _, gotName := range names {
			if gotName == n {
				t.Fatalf("owned %s in updates %v", n, names)
			}
		}
	}
	found := false
	for _, u := range got.Updates {
		if u.Name == "sonarr" {
			found = true
			if u.Auto != "24h" || u.Newest == nil || u.Newest.Tag != "4.0.16.2945-ls286" {
				t.Fatalf("sonarr %+v", u)
			}
		}
	}
	if !found {
		t.Fatalf("sonarr missing from %v", names)
	}
}

func TestWatchUpdatesExitsOnCancel(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	svc := &Service{Catalog: loadExamples(t), UpdateEvery: time.Hour, Images: &fakeList{}}
	done := make(chan struct{})
	go func() {
		svc.WatchUpdates(ctx)
		close(done)
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("WatchUpdates did not exit")
	}
}

func repoHeadHash(t *testing.T, dir string) string {
	t.Helper()
	_, hash := repoHead(t, dir)
	return hash
}
