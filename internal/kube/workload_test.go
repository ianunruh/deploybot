package kube

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestReadWorkloadDeploymentPods(t *testing.T) {
	t.Parallel()
	created := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	restarted := time.Date(2026, 8, 23, 10, 0, 0, 0, time.UTC)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /apis/apps/v1/namespaces/kmc-system/deployments/kmc", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"kind":     "Deployment",
			"metadata": map[string]any{"name": "kmc"},
			"spec": map[string]any{
				"replicas": 2,
				"selector": map[string]any{"matchLabels": map[string]string{"app.kubernetes.io/name": "kmc"}},
			},
			"status": map[string]any{
				"replicas": 2, "readyReplicas": 1, "updatedReplicas": 2, "availableReplicas": 1,
			},
		})
	})
	mux.HandleFunc("GET /api/v1/namespaces/kmc-system/pods", func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("labelSelector"); got != "app.kubernetes.io/name=kmc" {
			t.Errorf("selector %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []any{
				podJSON("kmc-aaa-1", "kmc-aaa", created, "worker-a", "10.0.0.1", "Running", 0, true, nil),
				podJSON("kmc-bbb-2", "kmc-bbb", created, "worker-b", "10.0.0.2", "CrashLoopBackOff", 3, false, &restarted),
				podJSON("other-1", "other-rs", created, "worker-a", "10.0.0.9", "Running", 0, true, nil),
			},
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	c := &REST{BaseURL: srv.URL, HTTP: srv.Client(), Auth: Bearer("t")}
	got := ReadWorkload(t.Context(), c, "kmc-system", "Deployment", "kmc")
	if got.Message != "" {
		t.Fatalf("message %q", got.Message)
	}
	if got.Kind != "Deployment" || got.Name != "kmc" || got.Desired != 2 || got.Ready != 1 || got.Updated != 2 {
		t.Fatalf("%+v", got)
	}
	if len(got.Pods) != 2 {
		t.Fatalf("pods %+v", got.Pods)
	}
	if got.Pods[0].Name != "kmc-aaa-1" || got.Pods[0].Ready != "1/1" || got.Pods[0].Status != "Running" {
		t.Fatalf("pod0 %+v", got.Pods[0])
	}
	if got.Pods[0].Node != "worker-a" || got.Pods[0].IP != "10.0.0.1" || got.Pods[0].Restarts != 0 {
		t.Fatalf("pod0 wide %+v", got.Pods[0])
	}
	if got.Pods[1].Status != "CrashLoopBackOff" || got.Pods[1].Restarts != 3 || got.Pods[1].RestartedAt == nil {
		t.Fatalf("pod1 %+v", got.Pods[1])
	}
	if !got.Pods[1].RestartedAt.Equal(restarted) {
		t.Fatalf("restarted %+v", got.Pods[1].RestartedAt)
	}
}

func TestReadWorkloadStatefulSetAndMissing(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /apis/apps/v1/namespaces/play/statefulsets/sonarr", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"kind":     "StatefulSet",
			"metadata": map[string]any{"name": "sonarr"},
			"spec": map[string]any{
				"replicas": 1,
				"selector": map[string]any{"matchLabels": map[string]string{"app.kubernetes.io/name": "sonarr"}},
			},
			"status": map[string]any{"readyReplicas": 1, "updatedReplicas": 1},
		})
	})
	mux.HandleFunc("GET /apis/apps/v1/namespaces/play/deployments/missing", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"reason":"NotFound"}`))
	})
	mux.HandleFunc("GET /api/v1/namespaces/play/pods", func(w http.ResponseWriter, r *http.Request) {
		sel := r.URL.Query().Get("labelSelector")
		items := []any{}
		if sel == "app.kubernetes.io/name=sonarr" {
			items = []any{
				map[string]any{
					"metadata": map[string]any{
						"name": "sonarr-0",
						"ownerReferences": []any{
							map[string]any{"kind": "StatefulSet", "name": "sonarr", "controller": true},
						},
					},
					"spec": map[string]any{
						"nodeName":   "n1",
						"containers": []any{map[string]any{"name": "web"}},
					},
					"status": map[string]any{
						"phase": "Running",
						"podIP": "10.1.0.8",
						"containerStatuses": []any{
							map[string]any{
								"name": "web", "ready": true, "restartCount": 1,
								"state": map[string]any{"running": map[string]any{"startedAt": "2026-08-23T00:00:00Z"}},
							},
						},
					},
				},
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"items": items})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	c := &REST{BaseURL: srv.URL, HTTP: srv.Client(), Auth: Bearer("t")}

	got := ReadWorkload(t.Context(), c, "play", "StatefulSet", "sonarr")
	if got.Kind != "StatefulSet" || got.Desired != 1 || got.Ready != 1 || len(got.Pods) != 1 {
		t.Fatalf("%+v", got)
	}
	if got.Pods[0].Name != "sonarr-0" || got.Pods[0].Status != "Running" {
		t.Fatalf("pod %+v", got.Pods[0])
	}

	missing := ReadWorkload(t.Context(), c, "play", "Deployment", "missing")
	if missing.Message != "Deployment/missing not found" {
		t.Fatalf("missing %q", missing.Message)
	}
	if len(missing.Pods) != 0 {
		t.Fatalf("missing pods %+v", missing.Pods)
	}
}

func TestPodStatusReasons(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		raw    string
		status string
		ready  string
	}{
		{
			name: "waiting",
			raw: `{
				"spec": {"containers": [{"name": "web"}]},
				"status": {
					"phase": "Pending",
					"containerStatuses": [{"state": {"waiting": {"reason": "ContainerCreating"}}}]
				}
			}`,
			status: "ContainerCreating",
			ready:  "0/1",
		},
		{
			name: "terminating",
			raw: `{
				"metadata": {"deletionTimestamp": "2026-08-23T00:00:00Z"},
				"status": {"phase": "Running"}
			}`,
			status: "Terminating",
			ready:  "0/0",
		},
		{
			name: "init",
			raw: `{
				"spec": {"initContainers": [{"name": "init"}], "containers": [{"name": "web"}]},
				"status": {
					"phase": "Pending",
					"initContainerStatuses": [{"state": {"waiting": {"reason": "PodInitializing"}}}]
				}
			}`,
			status: "Init:0/1",
			ready:  "0/1",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var p pod
			if err := json.Unmarshal([]byte(tc.raw), &p); err != nil {
				t.Fatal(err)
			}
			if got := podStatus(p); got != tc.status {
				t.Fatalf("status %q", got)
			}
			if got := podReady(p); got != tc.ready {
				t.Fatalf("ready %q", got)
			}
		})
	}
}

func TestLabelSelectorStable(t *testing.T) {
	t.Parallel()
	got := labelSelector(map[string]string{"b": "2", "a": "1"})
	if got != "a=1,b=2" {
		t.Fatalf("%q", got)
	}
}

func podJSON(name, rs string, created time.Time, node, ip, waiting string, restarts int, ready bool, restarted *time.Time) map[string]any {
	state := map[string]any{}
	if waiting == "Running" {
		state["running"] = map[string]any{"startedAt": created.Format(time.RFC3339)}
	} else {
		state["waiting"] = map[string]any{"reason": waiting}
	}
	last := map[string]any{}
	if restarted != nil {
		last["terminated"] = map[string]any{"finishedAt": restarted.Format(time.RFC3339), "exitCode": 1}
	}
	return map[string]any{
		"metadata": map[string]any{
			"name":              name,
			"creationTimestamp": created.Format(time.RFC3339),
			"ownerReferences": []any{
				map[string]any{"kind": "ReplicaSet", "name": rs, "controller": true},
			},
		},
		"spec": map[string]any{
			"nodeName":   node,
			"containers": []any{map[string]any{"name": "web"}},
		},
		"status": map[string]any{
			"phase": "Running",
			"podIP": ip,
			"containerStatuses": []any{
				map[string]any{
					"name": "web", "ready": ready, "restartCount": restarts,
					"state": state, "lastState": last,
				},
			},
		},
	}
}
