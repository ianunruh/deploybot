package release

import (
	"net/url"
	"strings"

	"github.com/ianunruh/deploybot/internal/config"
)

const (
	grafanaComputeDashboardID = "a87fb0d919ec0ea5f6543124e16c42a5"
	grafanaComputeDashboard   = "kubernetes-compute-resources-namespace-workloads"
	grafanaLogsDatasource     = "vZKdCamNk"
	headlampCluster           = "main"
)

// ObservabilityURLs builds Headlamp and Grafana links from cluster UI origins.
// Paths and query params are the same across clusters. Loki log drilldown is
// emitted only when Grafana.Logs is set.
func ObservabilityURLs(cluster config.Cluster, namespace string) (headlamp, grafana, logs string) {
	if namespace == "" {
		return "", "", ""
	}
	headlamp = headlampDeploymentsURL(cluster.Headlamp.URL, namespace)
	grafana = grafanaNamespaceWorkloadsURL(cluster.Grafana.URL, namespace)
	if cluster.Grafana.Logs {
		logs = grafanaNamespaceLogsURL(cluster.Grafana.URL, namespace)
	}
	return headlamp, grafana, logs
}

func headlampDeploymentsURL(base, namespace string) string {
	q := url.Values{}
	q.Set("namespace", namespace)
	return originURL(base, "/c/"+headlampCluster+"/deployments", q)
}

func grafanaNamespaceWorkloadsURL(base, namespace string) string {
	q := url.Values{}
	q.Set("from", "now-1h")
	q.Set("to", "now")
	q.Set("var-namespace", namespace)
	return originURL(base, "/d/"+grafanaComputeDashboardID+"/"+grafanaComputeDashboard, q)
}

func grafanaNamespaceLogsURL(base, namespace string) string {
	path, err := url.JoinPath("/a/grafana-lokiexplore-app/explore/namespace", namespace, "logs")
	if err != nil {
		path = "/a/grafana-lokiexplore-app/explore/namespace/" + namespace + "/logs"
	}
	q := url.Values{}
	q.Set("from", "now-15m")
	q.Set("to", "now")
	q.Set("var-ds", grafanaLogsDatasource)
	q.Set("var-filters", "namespace|=|"+namespace)
	return originURL(base, path, q)
}

func originURL(base, path string, q url.Values) string {
	base = strings.TrimSpace(base)
	if base == "" {
		return ""
	}
	u, err := url.Parse(base)
	if err != nil || u.Host == "" {
		return ""
	}
	if u.Scheme == "" {
		u.Scheme = "https"
	}
	u.Path = path
	u.RawQuery = q.Encode()
	u.Fragment = ""
	return u.String()
}
