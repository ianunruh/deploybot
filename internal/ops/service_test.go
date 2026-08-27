package ops

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ianunruh/deploybot/internal/kube"
)

func TestCatalog(t *testing.T) {
	t.Parallel()
	c := (&Service{Names: []string{"prod", "homelab"}, Config: Config{Ref: "main", Image: "img"}}).Catalog()
	if len(c.Kinds) == 0 || c.Kinds[0].Name != KindPyinfra {
		t.Fatalf("kinds %+v", c.Kinds)
	}
	if c.Kinds[0].Fields[0].Type != FieldMulti {
		t.Fatalf("fields %+v", c.Kinds[0].Fields)
	}
	if strings.Join(c.Clusters, ",") != "homelab,prod" {
		t.Fatalf("clusters %v", c.Clusters)
	}
	if !c.ImageSet || c.DefaultRef != "main" {
		t.Fatalf("%+v", c)
	}
}

func TestStartListGetLogs(t *testing.T) {
	var created atomic.Int32
	jobs := map[string]batchJob{}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /apis/batch/v1/namespaces/ops-ci/jobs", func(w http.ResponseWriter, r *http.Request) {
		created.Add(1)
		var job batchJob
		if err := json.NewDecoder(r.Body).Decode(&job); err != nil {
			t.Fatal(err)
		}
		if job.Metadata.GenerateName != generateName {
			t.Errorf("generateName %q", job.Metadata.GenerateName)
		}
		if job.Spec.Template.Spec.Containers[0].Args[0] != "pyinfra" {
			t.Errorf("args %v", job.Spec.Template.Spec.Containers[0].Args)
		}
		if job.Spec.Template.Spec.Containers[0].Command[0] != entrypoint {
			t.Errorf("command %v", job.Spec.Template.Spec.Containers[0].Command)
		}
		if *job.Spec.BackoffLimit != 0 {
			t.Errorf("backoff %v", job.Spec.BackoffLimit)
		}
		job.Metadata.Name = "ops-test1"
		now := time.Now().UTC()
		job.Metadata.CreationTimestamp = &now
		jobs[job.Metadata.Name] = job
		_ = json.NewEncoder(w).Encode(job)
	})
	mux.HandleFunc("GET /apis/batch/v1/namespaces/ops-ci/jobs", func(w http.ResponseWriter, r *http.Request) {
		sel := r.URL.Query().Get("labelSelector")
		if !strings.Contains(sel, "deploybot.io/execution=true") {
			t.Errorf("selector %q", sel)
		}
		var items []batchJob
		for _, j := range jobs {
			items = append(items, j)
		}
		_ = json.NewEncoder(w).Encode(jobList{Items: items})
	})
	mux.HandleFunc("GET /apis/batch/v1/namespaces/ops-ci/jobs/ops-test1", func(w http.ResponseWriter, _ *http.Request) {
		j := jobs["ops-test1"]
		j.Status.Active = 1
		_ = json.NewEncoder(w).Encode(j)
	})
	mux.HandleFunc("GET /api/v1/namespaces/ops-ci/pods", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("labelSelector") != "job-name=ops-test1" {
			t.Errorf("pod selector %q", r.URL.Query().Get("labelSelector"))
		}
		_ = json.NewEncoder(w).Encode(podList{Items: []pod{{
			Metadata: objectMeta{Name: "ops-test1-pod"},
			Status:   podStatus{Phase: "Running"},
		}}})
	})
	mux.HandleFunc("GET /api/v1/namespaces/ops-ci/pods/ops-test1-pod/log", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("container") != "ops" {
			t.Errorf("container %q", r.URL.Query().Get("container"))
		}
		_, _ = w.Write([]byte("hello from pyinfra\n"))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	svc := &Service{
		Config: Config{Image: "ghcr.io/ianunruh/kcloud-ops@sha256:abc", RepoURL: "https://github.com/ianunruh/kcloud-ops"},
		Kube:   map[string]*kube.REST{"homelab": {BaseURL: srv.URL, HTTP: srv.Client()}},
		Names:  []string{"homelab"},
		Actor:  Actor{Kind: "user", ID: "ian"},
	}
	dry := true
	ex, err := svc.Start(t.Context(), Request{
		Kind:    KindPyinfra,
		Cluster: "homelab",
		DryRun:  &dry,
		Params:  json.RawMessage(`{"roles":["common"],"limit":"exporter_nodes"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if ex.ID != "ops-test1" || ex.Kind != KindPyinfra || !ex.DryRun || ex.Summary != "common @ exporter_nodes" {
		t.Fatalf("%+v", ex)
	}
	if ex.Actor.ID != "ian" {
		t.Fatalf("actor %+v", ex.Actor)
	}

	listed, err := svc.List(t.Context(), "", "homelab")
	if err != nil || len(listed) != 1 {
		t.Fatalf("list %v %v", listed, err)
	}
	got, err := svc.Get(t.Context(), "homelab", "ops-test1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Phase != PhaseRunning || got.PodName != "ops-test1-pod" {
		t.Fatalf("%+v", got)
	}
	rc, err := svc.Logs(t.Context(), "homelab", "ops-test1", false)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(rc)
	_ = rc.Close()
	if string(body) != "hello from pyinfra\n" {
		t.Fatalf("logs %q", body)
	}

	_, err = svc.Start(t.Context(), Request{
		Kind:    KindPyinfra,
		Cluster: "homelab",
		Params:  json.RawMessage(`{"roles":["common"],"limit":"exporter_nodes"}`),
	})
	if !errors.Is(err, ErrBusy) {
		t.Fatalf("busy got %v", err)
	}
}

func TestStartRequiresImageAndKind(t *testing.T) {
	t.Parallel()
	svc := &Service{Names: []string{"homelab"}, Kube: map[string]*kube.REST{"homelab": {}}}
	_, err := svc.Start(t.Context(), Request{Kind: KindPyinfra, Cluster: "homelab", Params: json.RawMessage(`{"roles":["common"]}`)})
	if !errors.Is(err, ErrNoImage) {
		t.Fatalf("image %v", err)
	}
	svc.Config.Image = "img"
	_, err = svc.Start(t.Context(), Request{Kind: "nope", Cluster: "homelab"})
	if err == nil || !strings.Contains(err.Error(), "unknown ops kind") {
		t.Fatalf("kind %v", err)
	}
}

func TestBuildJobAvoidNode(t *testing.T) {
	t.Parallel()
	k := Kind{
		Name:     "replace_node",
		WorkDir:  "deploys",
		Deadline: time.Hour,
		Summary:  func(json.RawMessage) string { return "replace" },
		AvoidNode: func(_ string, _ json.RawMessage) string {
			return "controlplane102"
		},
	}
	job := buildJob(Config{Image: "img", RepoURL: "https://git"}, "homelab", k, Request{Kind: "replace_node"}, []string{"python", "replace_node.py"}, Actor{})
	aff := job.Spec.Template.Spec.Affinity
	if aff == nil || aff.NodeAffinity == nil {
		t.Fatal("expected affinity")
	}
	vals := aff.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions[0].Values
	if vals[0] != "controlplane102" {
		t.Fatalf("%v", vals)
	}
	uid := job.Spec.Template.Spec.SecurityContext.RunAsUser
	if uid == nil || *uid != 1000 {
		t.Fatalf("runAsUser %v", uid)
	}
	cuid := job.Spec.Template.Spec.Containers[0].SecurityContext.RunAsUser
	if cuid == nil || *cuid != 1000 {
		t.Fatalf("container runAsUser %v", cuid)
	}
}
