package release

import (
	"net/url"
	"strings"
)

const (
	grafanaComputeDashboardID = "a87fb0d919ec0ea5f6543124e16c42a5"
	grafanaComputeDashboard   = "kubernetes-compute-resources-namespace-workloads"
	grafanaLogsDatasource     = "vZKdCamNk"
	headlampCluster           = "main"
)

// ObservabilityURLs builds Headlamp and Grafana links for a stage.
// Homelab is kcloud.zone, prod is kcloud.io. Loki log drilldown exists
// only on homelab. The compute dashboard UID is the same on both Grafanas.
func ObservabilityURLs(stage, namespace string) (headlamp, grafana, logs string) {
	host := clusterPublicHost(stage)
	if host == "" || namespace == "" {
		return "", "", ""
	}
	headlamp = headlampDeploymentsURL(host, namespace)
	grafana = grafanaNamespaceWorkloadsURL(host, namespace)
	if clusterHasLogs(stage) {
		logs = grafanaNamespaceLogsURL(host, namespace)
	}
	return headlamp, grafana, logs
}

func clusterPublicHost(stage string) string {
	switch strings.ToLower(strings.TrimSpace(stage)) {
	case "homelab":
		return "k8s.kcloud.zone"
	case "prod":
		return "k8s.kcloud.io"
	default:
		return ""
	}
}

func clusterHasLogs(stage string) bool {
	return strings.EqualFold(strings.TrimSpace(stage), "homelab")
}

func headlampDeploymentsURL(host, namespace string) string {
	u := url.URL{
		Scheme: "https",
		Host:   "headlamp." + host,
		Path:   "/c/" + headlampCluster + "/deployments",
	}
	q := url.Values{}
	q.Set("namespace", namespace)
	u.RawQuery = q.Encode()
	return u.String()
}

func grafanaNamespaceWorkloadsURL(host, namespace string) string {
	u := url.URL{
		Scheme: "https",
		Host:   "grafana." + host,
		Path:   "/d/" + grafanaComputeDashboardID + "/" + grafanaComputeDashboard,
	}
	q := url.Values{}
	q.Set("from", "now-1h")
	q.Set("to", "now")
	q.Set("var-namespace", namespace)
	u.RawQuery = q.Encode()
	return u.String()
}

func grafanaNamespaceLogsURL(host, namespace string) string {
	path, err := url.JoinPath("/a/grafana-lokiexplore-app/explore/namespace", namespace, "logs")
	if err != nil {
		path = "/a/grafana-lokiexplore-app/explore/namespace/" + namespace + "/logs"
	}
	u := url.URL{
		Scheme: "https",
		Host:   "grafana." + host,
		Path:   path,
	}
	q := url.Values{}
	q.Set("from", "now-15m")
	q.Set("to", "now")
	q.Set("var-ds", grafanaLogsDatasource)
	q.Set("var-filters", "namespace|=|"+namespace)
	u.RawQuery = q.Encode()
	return u.String()
}
