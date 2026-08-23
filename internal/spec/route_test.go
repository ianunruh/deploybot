package spec

import (
	"strings"
	"testing"
)

func TestParseControllerOmitsRoute(t *testing.T) {
	t.Parallel()
	d, err := Parse([]byte(controllerYAML))
	if err != nil {
		t.Fatal(err)
	}
	if d.HasRoute() {
		t.Fatal("controller spec should not have a route")
	}
	if d.Spec.Workload.ContainerPort != 0 {
		t.Fatalf("containerPort %d", d.Spec.Workload.ContainerPort)
	}
	if d.Spec.Workload.PortName != "" {
		t.Fatalf("portName defaulted to %q", d.Spec.Workload.PortName)
	}
	if d.Spec.Route.GatewayNamespace != "" {
		t.Fatalf("gateway namespace defaulted to %q", d.Spec.Route.GatewayNamespace)
	}
	st, err := d.Stage("homelab")
	if err != nil {
		t.Fatal(err)
	}
	if st.Hostname != "" || st.Gateway.Name != "" {
		t.Fatalf("stage %+v", st)
	}
}

func TestRouteStillRequiresHostname(t *testing.T) {
	t.Parallel()
	body := strings.ReplaceAll(kmcYAML, "hostname: console.kcloud.zone", "hostname: \"\"")
	if _, err := Parse([]byte(body)); err == nil {
		t.Fatal("expected hostname required when a route is configured")
	}
}

func TestRouteRequiresPort(t *testing.T) {
	t.Parallel()
	body := strings.ReplaceAll(kmcYAML, "containerPort: 3000\n", "")
	if _, err := Parse([]byte(body)); err == nil {
		t.Fatal("expected port required when a route is configured")
	}
}
