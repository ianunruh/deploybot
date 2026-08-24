package release

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ianunruh/deploybot/internal/argo"
	"github.com/ianunruh/deploybot/internal/gitwrite"
	"github.com/ianunruh/deploybot/internal/image"
	"github.com/ianunruh/deploybot/internal/kube"
)

func TestStatusLinksAndArgoURL(t *testing.T) {
	t.Parallel()
	homelab := argo.NewFake()
	homelab.UIBase = "https://argocd.k8s.kcloud.zone"
	homelabAt := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	prodAt := time.Date(2026, 8, 23, 15, 4, 0, 0, time.UTC)
	homelab.Set("kmc", argo.Status{Health: "Healthy", Sync: "Synced", DeployedAt: &homelabAt})
	prod := argo.NewFake()
	prod.UIBase = "https://argocd.k8s.kcloud.io"
	prod.Set("kmc", argo.Status{Health: "Healthy", Sync: "Synced", DeployedAt: &prodAt})
	svc := &Service{
		Catalog:  loadExamples(t),
		Argo:     stageRouter{"homelab": homelab, "prod": prod},
		Clusters: testClusters(),
		Sync:     true,
	}
	st, err := svc.Status(t.Context(), "kmc")
	if err != nil {
		t.Fatal(err)
	}
	if !st.Sync {
		t.Fatalf("expected sync flag, got %+v", st)
	}
	if st.RepoURL != "https://github.com/ianunruh/kmc" {
		t.Fatalf("repo %q", st.RepoURL)
	}
	if st.ProjectURL != "https://trello.com/b/rPALXxJF/kcloud" {
		t.Fatalf("project %q", st.ProjectURL)
	}
	if len(st.Stages) != 2 {
		t.Fatalf("stages %+v", st.Stages)
	}
	if st.Stages[0].ArgoURL != "https://argocd.k8s.kcloud.zone/applications/kmc" {
		t.Fatalf("homelab argo %q", st.Stages[0].ArgoURL)
	}
	if st.Stages[1].ArgoURL != "https://argocd.k8s.kcloud.io/applications/kmc" {
		t.Fatalf("prod argo %q", st.Stages[1].ArgoURL)
	}
	if st.Stages[0].HeadlampURL == "" || st.Stages[0].GrafanaURL == "" || st.Stages[0].LogsURL == "" {
		t.Fatalf("homelab observability %+v", st.Stages[0])
	}
	if st.Stages[1].HeadlampURL == "" || st.Stages[1].GrafanaURL == "" || st.Stages[1].LogsURL != "" {
		t.Fatalf("prod observability %+v", st.Stages[1])
	}
	assertURL(t, "status headlamp", st.Stages[0].HeadlampURL, "https://headlamp.k8s.kcloud.zone/c/main/deployments?namespace=kmc-system")
	assertURL(t, "status grafana", st.Stages[1].GrafanaURL, "https://grafana.k8s.kcloud.io/d/a87fb0d919ec0ea5f6543124e16c42a5/kubernetes-compute-resources-namespace-workloads?from=now-1h&to=now&var-namespace=kmc-system")
	if st.Stages[0].DeployedAt == nil || !st.Stages[0].DeployedAt.Equal(homelabAt) {
		t.Fatalf("homelab deployedAt %+v", st.Stages[0].DeployedAt)
	}
	if st.Stages[1].DeployedAt == nil || !st.Stages[1].DeployedAt.Equal(prodAt) {
		t.Fatalf("prod deployedAt %+v", st.Stages[1].DeployedAt)
	}
	if len(st.Flow.Hops) != 1 || st.Flow.Hops[0].From != "homelab" || st.Flow.Hops[0].To != "prod" {
		t.Fatalf("flow %+v", st.Flow)
	}
	if st.Flow.Hops[0].State != HopCaughtUp {
		t.Fatalf("unpinned stages should be caught up, got %q", st.Flow.Hops[0].State)
	}
	if st.Stages[0].Workload != nil || st.Stages[1].Workload != nil {
		t.Fatalf("fake argo should not attach workload %+v", st.Stages)
	}

	latest := svc.Latest(t.Context())
	kmc := latest["kmc"]
	if kmc.DeployedAt == nil || !kmc.DeployedAt.Equal(prodAt) {
		t.Fatalf("latest kmc deployedAt %+v", kmc.DeployedAt)
	}
	if len(kmc.Flow.Hops) != 1 || kmc.Flow.Hops[0].State != HopCaughtUp {
		t.Fatalf("latest kmc flow %+v", kmc.Flow)
	}
	if len(kmc.Stages) != 2 || kmc.Stages[0].Health != "Healthy" {
		t.Fatalf("latest kmc stages %+v", kmc.Stages)
	}
}

func TestLatestWaitingApproval(t *testing.T) {
	t.Parallel()
	homelab := argo.NewFake()
	homelab.Set("kmc", argo.Status{Health: "Healthy", Sync: "Synced"})
	prod := argo.NewFake()
	prod.Set("kmc", argo.Status{Health: "Healthy", Sync: "Synced"})
	svc := &Service{
		Catalog: loadExamples(t),
		OpsRepo: initOpsRepo(t),
		Apply:   true,
		Argo:    stageRouter{"homelab": homelab, "prod": prod},
		Author:  gitwrite.Author{Name: "t", Email: "t@t"},
	}
	if _, err := svc.Pin(t.Context(), "kmc", "homelab", "ghcr.io/ianunruh/kmc@sha256:abc"); err != nil {
		t.Fatal(err)
	}
	latest := svc.Latest(t.Context())
	kmc := latest["kmc"]
	if len(kmc.Flow.Hops) != 1 || kmc.Flow.Hops[0].State != HopWaitingApproval {
		t.Fatalf("flow %+v", kmc.Flow)
	}
	if len(kmc.Stages) != 2 || kmc.Stages[0].Image == "" {
		t.Fatalf("homelab image %+v", kmc.Stages)
	}
}

func TestLatestListsOncePerStage(t *testing.T) {
	t.Parallel()
	homelab := argo.NewFake()
	homelab.Set("kmc", argo.Status{Health: "Healthy", Sync: "Synced"})
	homelab.Set("play-sonarr", argo.Status{Health: "Healthy", Sync: "Synced"})
	prod := argo.NewFake()
	prod.Set("kmc", argo.Status{Health: "Healthy", Sync: "Synced"})
	svc := &Service{
		Catalog: loadExamples(t),
		Argo:    stageRouter{"homelab": homelab, "prod": prod},
		appsTTL: time.Minute,
	}
	latest := svc.Latest(t.Context())
	if latest["kmc"].Stages[0].Health != "Healthy" {
		t.Fatalf("kmc %+v", latest["kmc"])
	}
	if latest["sonarr"].Stages[0].Health != "Healthy" {
		t.Fatalf("sonarr %+v", latest["sonarr"])
	}
	hGet, hList := homelab.Calls()
	pGet, pList := prod.Calls()
	if hGet != 0 || pGet != 0 {
		t.Fatalf("status should list, not get: homelab get %d prod get %d", hGet, pGet)
	}
	if hList != 1 || pList != 1 {
		t.Fatalf("lists homelab %d prod %d", hList, pList)
	}
	_ = svc.Latest(t.Context())
	_, hList = homelab.Calls()
	_, pList = prod.Calls()
	if hList != 1 || pList != 1 {
		t.Fatalf("ttl should skip list: homelab %d prod %d", hList, pList)
	}
	svc.dropArgo("homelab")
	_ = svc.Latest(t.Context())
	_, hList = homelab.Calls()
	_, pList = prod.Calls()
	if hList != 2 || pList != 1 {
		t.Fatalf("drop homelab: homelab %d prod %d", hList, pList)
	}
}

func TestLiveStatusDropsArgoCache(t *testing.T) {
	t.Parallel()
	homelab := argo.NewFake()
	homelab.Set("kmc", argo.Status{Health: "Healthy", Sync: "Synced"})
	prod := argo.NewFake()
	prod.Set("kmc", argo.Status{Health: "Healthy", Sync: "Synced"})
	svc := &Service{
		Catalog: loadExamples(t),
		Argo:    stageRouter{"homelab": homelab, "prod": prod},
		appsTTL: time.Minute,
	}
	if _, err := svc.Status(t.Context(), "kmc"); err != nil {
		t.Fatal(err)
	}
	_, hList := homelab.Calls()
	_, pList := prod.Calls()
	if hList != 1 || pList != 1 {
		t.Fatalf("first status lists homelab %d prod %d", hList, pList)
	}
	if _, err := svc.Status(t.Context(), "kmc"); err != nil {
		t.Fatal(err)
	}
	_, hList = homelab.Calls()
	_, pList = prod.Calls()
	if hList != 1 || pList != 1 {
		t.Fatalf("cached status listed again: homelab %d prod %d", hList, pList)
	}
	if _, err := svc.LiveStatus(t.Context(), "kmc"); err != nil {
		t.Fatal(err)
	}
	_, hList = homelab.Calls()
	_, pList = prod.Calls()
	if hList != 2 || pList != 2 {
		t.Fatalf("live status should relist: homelab %d prod %d", hList, pList)
	}
}

func TestListImages(t *testing.T) {
	t.Parallel()
	cat := loadExamples(t)
	svc := &Service{
		Catalog: cat,
		Images: fakeImages{listing: image.Listing{
			Source: "ghcr",
			Versions: []image.Version{{
				Repository: "ghcr.io/ianunruh/kmc",
				Ref:        "ghcr.io/ianunruh/kmc:main-b8e5098@sha256:abc",
				Tag:        "main-b8e5098",
			}},
		}},
	}
	got, err := svc.ListImages(t.Context(), "kmc")
	if err != nil {
		t.Fatal(err)
	}
	if got.Source != "ghcr" || got.Repository != "ghcr.io/ianunruh/kmc" || len(got.Images) != 1 {
		t.Fatalf("%+v", got)
	}
	if _, err := svc.ListImages(t.Context(), "nope"); err == nil {
		t.Fatal("expected unknown deployable")
	}
	if _, err := (&Service{Catalog: cat}).ListImages(t.Context(), "kmc"); err == nil {
		t.Fatal("expected unconfigured listing")
	}
}

func TestStatusLiveWorkload(t *testing.T) {
	t.Parallel()
	var podLists atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("GET /apis/argoproj.io/v1alpha1/namespaces/argocd/applications", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []any{
				map[string]any{
					"metadata": map[string]any{"name": "kmc"},
					"status": map[string]any{
						"health": map[string]any{"status": "Healthy"},
						"sync":   map[string]any{"status": "Synced"},
					},
				},
			},
		})
	})
	mux.HandleFunc("GET /apis/apps/v1/namespaces/kmc-system/deployments/kmc", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"kind":     "Deployment",
			"metadata": map[string]any{"name": "kmc"},
			"spec": map[string]any{
				"replicas": 1,
				"selector": map[string]any{"matchLabels": map[string]string{"app.kubernetes.io/name": "kmc"}},
			},
			"status": map[string]any{"readyReplicas": 1, "updatedReplicas": 1, "availableReplicas": 1},
		})
	})
	mux.HandleFunc("GET /api/v1/namespaces/kmc-system/pods", func(w http.ResponseWriter, r *http.Request) {
		podLists.Add(1)
		if got := r.URL.Query().Get("labelSelector"); got != "app.kubernetes.io/name=kmc" {
			t.Errorf("selector %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []any{
				map[string]any{
					"metadata": map[string]any{
						"name":              "kmc-abc-1",
						"creationTimestamp": "2026-08-20T12:00:00Z",
						"ownerReferences": []any{
							map[string]any{"kind": "ReplicaSet", "name": "kmc-abc", "controller": true},
						},
					},
					"spec": map[string]any{
						"nodeName":   "worker-1",
						"containers": []any{map[string]any{"name": "web"}},
					},
					"status": map[string]any{
						"phase": "Running",
						"podIP": "10.42.0.8",
						"containerStatuses": []any{
							map[string]any{
								"name": "web", "ready": true, "restartCount": 2,
								"state": map[string]any{"running": map[string]any{"startedAt": "2026-08-20T12:00:00Z"}},
							},
						},
					},
				},
			},
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	rest := &kube.REST{BaseURL: srv.URL, HTTP: srv.Client(), Auth: kube.Bearer("t")}
	client := &argo.KubeClient{REST: rest, Namespace: "argocd", UIBaseURL: "https://argocd.example"}
	svc := &Service{
		Catalog: loadExamples(t),
		Argo:    stageRouter{"homelab": client, "prod": client},
	}
	st, err := svc.Status(t.Context(), "kmc")
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Stages) != 2 {
		t.Fatalf("stages %+v", st.Stages)
	}
	for _, stage := range st.Stages {
		if stage.Health != "Healthy" || stage.Sync != "Synced" {
			t.Fatalf("argo %+v", stage)
		}
		if stage.Workload != nil {
			t.Fatalf("status should skip pods %+v", stage.Workload)
		}
	}
	if podLists.Load() != 0 {
		t.Fatalf("status listed pods")
	}

	live, err := svc.LiveWorkloads(t.Context(), "kmc")
	if err != nil {
		t.Fatal(err)
	}
	if len(live.Stages) != 2 {
		t.Fatalf("live stages %+v", live.Stages)
	}
	for _, stage := range live.Stages {
		wl := stage.Workload
		if wl == nil || wl.Kind != "Deployment" || wl.Ready != 1 || wl.Desired != 1 {
			t.Fatalf("workload %+v", wl)
		}
		if len(wl.Pods) != 1 || wl.Pods[0].Name != "kmc-abc-1" {
			t.Fatalf("pods %+v", wl.Pods)
		}
		p := wl.Pods[0]
		if p.Status != "Running" || p.Ready != "1/1" || p.Restarts != 2 || p.Node != "worker-1" || p.IP != "10.42.0.8" {
			t.Fatalf("pod %+v", p)
		}
	}
	if podLists.Load() != 2 {
		t.Fatalf("pod lists %d", podLists.Load())
	}

	before := podLists.Load()
	latest := svc.Latest(t.Context())
	kmc := latest["kmc"]
	if len(kmc.Stages) != 2 || kmc.Stages[0].Health != "Healthy" {
		t.Fatalf("latest %+v", kmc)
	}
	if kmc.Stages[0].Workload != nil {
		t.Fatalf("latest should skip pods %+v", kmc.Stages[0].Workload)
	}
	if podLists.Load() != before {
		t.Fatalf("latest listed pods")
	}

	if _, err := svc.LiveWorkloads(t.Context(), "nope"); err == nil {
		t.Fatal("expected unknown deployable")
	}
}

type fakeImages struct {
	listing image.Listing
	err     error
}

func (f fakeImages) List(_ context.Context, _ string, _ string) (image.Listing, error) {
	return f.listing, f.err
}
