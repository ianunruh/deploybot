package release

import (
	"net/url"
	"testing"

	"github.com/ianunruh/deploybot/internal/config"
)

func TestObservabilityURLs(t *testing.T) {
	t.Parallel()

	headlamp, grafana, logs := ObservabilityURLs(testClusters()["homelab"], "kmc-system")
	assertURL(t, "homelab headlamp", headlamp, "https://headlamp.k8s.kcloud.zone/c/main/deployments?namespace=kmc-system")
	assertURL(t, "homelab grafana", grafana, "https://grafana.k8s.kcloud.zone/d/a87fb0d919ec0ea5f6543124e16c42a5/kubernetes-compute-resources-namespace-workloads?from=now-1h&to=now&var-namespace=kmc-system")
	assertURL(t, "homelab logs", logs, "https://grafana.k8s.kcloud.zone/a/grafana-lokiexplore-app/explore/namespace/kmc-system/logs?from=now-15m&to=now&var-ds=vZKdCamNk&var-filters=namespace%7C%3D%7Ckmc-system")

	headlamp, grafana, logs = ObservabilityURLs(testClusters()["prod"], "kmc-system")
	assertURL(t, "prod headlamp", headlamp, "https://headlamp.k8s.kcloud.io/c/main/deployments?namespace=kmc-system")
	assertURL(t, "prod grafana", grafana, "https://grafana.k8s.kcloud.io/d/a87fb0d919ec0ea5f6543124e16c42a5/kubernetes-compute-resources-namespace-workloads?from=now-1h&to=now&var-namespace=kmc-system")
	if logs != "" {
		t.Fatalf("prod logs %q", logs)
	}

	headlamp, grafana, logs = ObservabilityURLs(testClusters()["homelab"], "alloy")
	assertURL(t, "alloy grafana", grafana, "https://grafana.k8s.kcloud.zone/d/a87fb0d919ec0ea5f6543124e16c42a5/kubernetes-compute-resources-namespace-workloads?from=now-1h&to=now&var-namespace=alloy")
	if headlamp == "" || logs == "" {
		t.Fatalf("alloy homelab missing links %q %q", headlamp, logs)
	}

	if h, g, l := ObservabilityURLs(config.Cluster{}, "kmc-system"); h != "" || g != "" || l != "" {
		t.Fatalf("unknown cluster: %q %q %q", h, g, l)
	}
	if h, g, l := ObservabilityURLs(testClusters()["homelab"], ""); h != "" || g != "" || l != "" {
		t.Fatalf("empty namespace: %q %q %q", h, g, l)
	}

	onlyHeadlamp := config.Cluster{Headlamp: config.Headlamp{URL: "https://headlamp.example"}}
	h, g, l := ObservabilityURLs(onlyHeadlamp, "ns")
	if h == "" || g != "" || l != "" {
		t.Fatalf("headlamp-only: %q %q %q", h, g, l)
	}

	grafanaNoLogs := config.Cluster{Grafana: config.Grafana{URL: "https://grafana.example"}}
	h, g, l = ObservabilityURLs(grafanaNoLogs, "ns")
	if h != "" || g == "" || l != "" {
		t.Fatalf("grafana without logs: %q %q %q", h, g, l)
	}
	logsOnly := config.Cluster{Grafana: config.Grafana{Logs: true}}
	if _, _, l = ObservabilityURLs(logsOnly, "ns"); l != "" {
		t.Fatalf("logs without grafana url: %q", l)
	}
}

func TestOriginURLRejectsBadBase(t *testing.T) {
	t.Parallel()
	q := url.Values{}
	q.Set("n", "x")
	if got := originURL("://bad", "/p", q); got != "" {
		t.Fatalf("bad url %q", got)
	}
	if got := originURL("not-a-host", "/p", q); got != "" {
		t.Fatalf("no host %q", got)
	}
}

func assertURL(t *testing.T, name, got, want string) {
	t.Helper()
	g, err := url.Parse(got)
	if err != nil {
		t.Fatalf("%s parse got: %v", name, err)
	}
	w, err := url.Parse(want)
	if err != nil {
		t.Fatalf("%s parse want: %v", name, err)
	}
	if g.Scheme != w.Scheme || g.Host != w.Host || g.EscapedPath() != w.EscapedPath() {
		t.Fatalf("%s origin got %s want %s", name, got, want)
	}
	if g.Query().Encode() != w.Query().Encode() {
		t.Fatalf("%s query got %s want %s", name, g.RawQuery, w.RawQuery)
	}
}
