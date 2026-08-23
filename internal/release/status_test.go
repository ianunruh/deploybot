package release

import (
	"context"
	"testing"
	"time"

	"github.com/ianunruh/deploybot/internal/argo"
	"github.com/ianunruh/deploybot/internal/image"
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
		Catalog: loadExamples(t),
		Argo:    stageRouter{"homelab": homelab, "prod": prod},
		Sync:    true,
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

	latest := svc.LatestDeployedAt(t.Context())
	if latest["kmc"] == nil || !latest["kmc"].Equal(prodAt) {
		t.Fatalf("latest kmc %+v", latest["kmc"])
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

type fakeImages struct {
	listing image.Listing
	err     error
}

func (f fakeImages) List(_ context.Context, _ string, _ string) (image.Listing, error) {
	return f.listing, f.err
}
