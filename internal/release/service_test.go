package release

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/ianunruh/deploybot/internal/argo"
	"github.com/ianunruh/deploybot/internal/catalog"
	botcfg "github.com/ianunruh/deploybot/internal/config"
	"github.com/ianunruh/deploybot/internal/gitwrite"
	"github.com/ianunruh/deploybot/internal/render"
	"github.com/ianunruh/deploybot/internal/spec"
)

func testClusters() map[string]botcfg.Cluster {
	return map[string]botcfg.Cluster{
		"homelab": {
			Headlamp: botcfg.Headlamp{URL: "https://headlamp.k8s.kcloud.zone"},
			Grafana:  botcfg.Grafana{URL: "https://grafana.k8s.kcloud.zone", Logs: true},
		},
		"prod": {
			Headlamp: botcfg.Headlamp{URL: "https://headlamp.k8s.kcloud.io"},
			Grafana:  botcfg.Grafana{URL: "https://grafana.k8s.kcloud.io"},
		},
	}
}

type stageRouter map[string]argo.Client

func (r stageRouter) ForStage(stage string) argo.Client { return r[stage] }

func mustOpenTree(t *testing.T, dir string) render.Tree {
	t.Helper()
	tree, err := gitwrite.OpenTree(dir)
	if err != nil {
		t.Fatal(err)
	}
	return tree
}

func mustKMC(t *testing.T, svc *Service) *spec.Deployable {
	t.Helper()
	d, err := svc.Catalog.Get("kmc")
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func loadExamples(t *testing.T) *catalog.Catalog {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(file), "..", "..", "examples")
	cat, err := catalog.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	return cat
}

func initOpsRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"README":                       "ops\n",
		"k8s/kmc/base/deployment.yaml": "KEEPME\n",
		"k8s/kmc/overlays/homelab/kustomization.yaml": "resources:\n  - ../../base\n",
		"k8s/kmc/overlays/prod/kustomization.yaml": `resources:
  - ../../base
configMapGenerator:
  - name: kmc-env
    envs:
      - config.env
`,
		"k8s/apps/projects/sandbox/overlays/homelab/kustomization.yaml": `resources:
  - ../../base
patches:
  - path: kmc.yaml
`,
	}
	for rel, body := range files {
		fp := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(fp), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fp, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := wt.Add(rel); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := wt.Commit("init", &git.CommitOptions{
		Author: &object.Signature{Name: "t", Email: "t@t", When: time.Now()},
	}); err != nil {
		t.Fatal(err)
	}
	return dir
}

func addBareRemote(t *testing.T, local string) string {
	t.Helper()
	remote := filepath.Join(t.TempDir(), "origin.git")
	if _, err := git.PlainInit(remote, true); err != nil {
		t.Fatal(err)
	}
	repo, err := git.PlainOpen(local)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CreateRemote(&config.RemoteConfig{
		Name: git.DefaultRemoteName,
		URLs: []string{remote},
	}); err != nil {
		t.Fatal(err)
	}
	if err := repo.Push(&git.PushOptions{RemoteName: git.DefaultRemoteName}); err != nil {
		t.Fatal(err)
	}
	return remote
}

func repoHead(t *testing.T, dir string) (branch, hash string) {
	t.Helper()
	repo, err := git.PlainOpen(dir)
	if err != nil {
		t.Fatal(err)
	}
	head, err := repo.Head()
	if err != nil {
		t.Fatal(err)
	}
	return head.Name().Short(), head.Hash().String()
}

func branchHash(t *testing.T, repoDir, branch string) string {
	t.Helper()
	repo, err := git.PlainOpen(repoDir)
	if err != nil {
		t.Fatal(err)
	}
	ref, err := repo.Reference(plumbing.NewBranchReferenceName(branch), true)
	if err != nil {
		t.Fatal(err)
	}
	return ref.Hash().String()
}

func initControllerOpsRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"README": "ops\n",
		"k8s/kmc-controller/base/kustomization.yaml": `resources:
  - deployment.yaml
  - crds/ipaddresses.yaml
  - rbac/service_account.yaml
patches:
  - path: patch-manager.yaml
`,
		"k8s/kmc-controller/base/crds/ipaddresses.yaml":     "kind: CustomResourceDefinition\n",
		"k8s/kmc-controller/base/rbac/service_account.yaml": "kind: ServiceAccount\n",
		"k8s/kmc-controller/base/patch-manager.yaml":        "kind: Deployment\n",
		"k8s/kmc-controller/overlays/homelab/kustomization.yaml": `resources:
  - ../../base
patches:
  - path: patch-cidrs.yaml
`,
		"k8s/kmc-controller/overlays/homelab/patch-cidrs.yaml": "kind: Deployment\n",
	}
	for rel, body := range files {
		fp := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(fp), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fp, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := wt.Add(rel); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := wt.Commit("init", &git.CommitOptions{
		Author: &object.Signature{Name: "t", Email: "t@t", When: time.Now()},
	}); err != nil {
		t.Fatal(err)
	}
	return dir
}
