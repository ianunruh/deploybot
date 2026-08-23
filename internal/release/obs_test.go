package release

import (
	"net/url"
	"testing"
)

func TestObservabilityURLs(t *testing.T) {
	t.Parallel()

	headlamp, grafana, logs := ObservabilityURLs("homelab", "kmc-system")
	assertURL(t, "homelab headlamp", headlamp, "https://headlamp.k8s.kcloud.zone/c/main/deployments?namespace=kmc-system")
	assertURL(t, "homelab grafana", grafana, "https://grafana.k8s.kcloud.zone/d/a87fb0d919ec0ea5f6543124e16c42a5/kubernetes-compute-resources-namespace-workloads?from=now-1h&to=now&var-namespace=kmc-system")
	assertURL(t, "homelab logs", logs, "https://grafana.k8s.kcloud.zone/a/grafana-lokiexplore-app/explore/namespace/kmc-system/logs?from=now-15m&to=now&var-ds=vZKdCamNk&var-filters=namespace%7C%3D%7Ckmc-system")

	headlamp, grafana, logs = ObservabilityURLs("prod", "kmc-system")
	assertURL(t, "prod headlamp", headlamp, "https://headlamp.k8s.kcloud.io/c/main/deployments?namespace=kmc-system")
	assertURL(t, "prod grafana", grafana, "https://grafana.k8s.kcloud.io/d/a87fb0d919ec0ea5f6543124e16c42a5/kubernetes-compute-resources-namespace-workloads?from=now-1h&to=now&var-namespace=kmc-system")
	if logs != "" {
		t.Fatalf("prod logs %q", logs)
	}

	headlamp, grafana, logs = ObservabilityURLs("homelab", "alloy")
	assertURL(t, "alloy grafana", grafana, "https://grafana.k8s.kcloud.zone/d/a87fb0d919ec0ea5f6543124e16c42a5/kubernetes-compute-resources-namespace-workloads?from=now-1h&to=now&var-namespace=alloy")
	if headlamp == "" || logs == "" {
		t.Fatalf("alloy homelab missing links %q %q", headlamp, logs)
	}

	if h, g, l := ObservabilityURLs("delta", "kmc-system"); h != "" || g != "" || l != "" {
		t.Fatalf("unknown stage: %q %q %q", h, g, l)
	}
	if h, g, l := ObservabilityURLs("homelab", ""); h != "" || g != "" || l != "" {
		t.Fatalf("empty namespace: %q %q %q", h, g, l)
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
