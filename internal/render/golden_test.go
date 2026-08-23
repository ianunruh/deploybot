package render

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ianunruh/deploybot/internal/spec"
	"github.com/ianunruh/deploybot/internal/yamlx"
)

func TestGoldens(t *testing.T) {
	t.Parallel()
	for _, name := range []string{
		"kmc", "kmc-controller", "deploybot", "deploybot-web",
		"sonarr", "radarr", "bazarr", "jackett", "tautulli", "ombi",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			d := loadExample(t, name)
			tree, err := Render(d)
			if err != nil {
				t.Fatal(err)
			}
			dir := filepath.Join(testDataDir(t), name)
			if os.Getenv("UPDATE_GOLDENS") == "1" {
				if err := writeTree(dir, tree); err != nil {
					t.Fatal(err)
				}
			}
			want, err := readTree(dir)
			if err != nil {
				t.Fatalf("read goldens (run UPDATE_GOLDENS=1 go test): %v", err)
			}
			if diff := cmp.Diff(stringMap(want), stringMap(tree)); diff != "" {
				t.Fatalf("golden mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestYAMLProbePort(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, want string
	}{
		{"8081", "port: 8081\n"},
		{"http", "port: http\n"},
	}
	for _, tc := range cases {
		b, err := yamlx.Marshal(httpGet{Path: "/", Port: yamlProbePort(tc.in)})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(b), tc.want) || strings.Contains(string(b), `port: "8081"`) {
			t.Fatalf("port %q marshaled as:\n%s", tc.in, b)
		}
	}
}

func TestControllerOmitsServiceAndRoute(t *testing.T) {
	t.Parallel()
	d := loadExample(t, "kmc-controller")
	tree, err := Render(d)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{
		"k8s/kmc-controller/base/service.yaml",
		"k8s/kmc-controller/base/httproute.yaml",
		"k8s/kmc-controller/overlays/prod/patch-httproute.yaml",
	} {
		if _, ok := tree[p]; ok {
			t.Fatalf("controller render emitted %s", p)
		}
	}
	kust := string(tree["k8s/kmc-controller/base/kustomization.yaml"])
	if strings.Contains(kust, "service.yaml") || strings.Contains(kust, "httproute.yaml") {
		t.Fatalf("base kustomization should only list deployment.yaml:\n%s", kust)
	}
	dep := string(tree["k8s/kmc-controller/base/deployment.yaml"])
	if strings.Contains(dep, "containerPort") {
		t.Fatalf("generated deployment should omit ports:\n%s", dep)
	}
	if !strings.Contains(dep, "path: /healthz") || !strings.Contains(dep, "path: /readyz") {
		t.Fatalf("generated deployment missing probes:\n%s", dep)
	}
	if strings.Contains(dep, `port: "8081"`) {
		t.Fatalf("numeric probe ports must marshal as integers, not quoted strings:\n%s", dep)
	}
	if !strings.Contains(dep, "port: 8081") {
		t.Fatalf("generated deployment missing numeric probe port:\n%s", dep)
	}
}

func TestPlayStatefulSetSkeleton(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"sonarr", "radarr", "bazarr", "jackett", "tautulli", "ombi"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			d := loadExample(t, name)
			tree, err := Render(d)
			if err != nil {
				t.Fatal(err)
			}
			base := "k8s/play/" + name + "/base/"
			if _, ok := tree[base+"deployment.yaml"]; ok {
				t.Fatal("play app should not emit deployment.yaml")
			}
			sts := string(tree[base+"statefulset.yaml"])
			if !strings.Contains(sts, "kind: StatefulSet") {
				t.Fatalf("missing StatefulSet:\n%s", sts)
			}
			for _, refuse := range []string{"PUID", "volumeMounts:", "volumeClaimTemplates:", "plex-media"} {
				if strings.Contains(sts, refuse) {
					t.Fatalf("generated statefulset must not include %q:\n%s", refuse, sts)
				}
			}
			if _, ok := tree["k8s/apps/projects/play/overlays/homelab/"+name+".yaml"]; ok {
				t.Fatal("argo overlay must use spec.argo.name, not metadata.name")
			}
			app := string(tree["k8s/apps/projects/play/overlays/homelab/play-"+name+".yaml"])
			if !strings.Contains(app, "name: play-"+name) {
				t.Fatalf("argo app:\n%s", app)
			}
			if !strings.Contains(app, "project: play") {
				t.Fatalf("argo project:\n%s", app)
			}
		})
	}
}

func TestKMCOmitsPodWiring(t *testing.T) {
	t.Parallel()
	d := loadExample(t, "kmc")
	tree, err := Render(d)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := tree["k8s/kmc/overlays/prod/patch-volumes.yaml"]; ok {
		t.Fatal("render must not emit patch-volumes.yaml")
	}
	dep := string(tree["k8s/kmc/base/deployment.yaml"])
	for _, refuse := range []string{"envFrom:", "KMC_CLUSTERS_CONFIG", "volumeMounts:", "kmc-clusters", "cluster-tokens"} {
		if strings.Contains(dep, refuse) {
			t.Fatalf("generated deployment must not include %q:\n%s", refuse, dep)
		}
	}
	prodKust := string(tree["k8s/kmc/overlays/prod/kustomization.yaml"])
	if strings.Contains(prodKust, "patch-volumes.yaml") {
		t.Fatalf("prod overlay must not list volume patch:\n%s", prodKust)
	}
}

func loadExample(t *testing.T, name string) *spec.Deployable {
	t.Helper()
	d, err := spec.Load(filepath.Join(repoRoot(t), "examples", name+".yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func testDataDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(filepath.Dir(funcFile()), "testdata")
}

func funcFile() string {
	_, file, _, _ := runtime.Caller(0)
	return file
}

func stringMap(t Tree) map[string]string {
	out := make(map[string]string, len(t))
	for p, b := range t {
		out[p] = string(b)
	}
	return out
}

func writeTree(dir string, tree Tree) error {
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	for p, b := range tree {
		fp := filepath.Join(dir, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(fp), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(fp, b, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func readTree(dir string) (Tree, error) {
	out := Tree{}
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		out[filepath.ToSlash(rel)] = b
		return nil
	})
	return out, err
}
