package ops

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

func TestPyinfraArgv(t *testing.T) {
	t.Parallel()
	params := json.RawMessage(`{"roles":["common","containerd","k8s"],"limit":"k8s_nodes","data":{"k8s_package_set":"kubeadm"}}`)
	if err := validatePyinfra("homelab", params); err != nil {
		t.Fatal(err)
	}
	got, err := argvPyinfra("homelab", true, params)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"pyinfra", "inventory.py",
		"common/deploy.py", "containerd/deploy.py", "k8s/deploy.py",
		"--limit", "k8s_nodes", "--dry", "-y",
		"--data", "k8s_package_set=kubeadm",
	}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("got %q", got)
	}
	prod, err := argvPyinfra("prod", false, json.RawMessage(`{"roles":["scrutiny"],"limit":"scrutiny_nodes"}`))
	if err != nil {
		t.Fatal(err)
	}
	if prod[1] != "inventory-prod.py" {
		t.Fatalf("inventory %q", prod[1])
	}
	if strings.Join(prod, " ") != "pyinfra inventory-prod.py scrutiny/deploy.py --limit scrutiny_nodes -y" {
		t.Fatalf("prod %q", prod)
	}
}

func TestPyinfraRejects(t *testing.T) {
	t.Parallel()
	cases := []string{
		`{}`,
		`{"roles":["common"]}`,
		`{"roles":["nope"]}`,
		`{"roles":["common","common"]}`,
		`{"roles":["common"],"limit":"not a host"}`,
		`{"roles":["common"],"data":{"nope":"x"}}`,
		`{"roles":["common"],"data":{"k8s_package_set":"nope"}}`,
		`{"roles":["common"],"extra":1}`,
	}
	for _, raw := range cases {
		if err := validatePyinfra("homelab", json.RawMessage(raw)); err == nil {
			t.Fatalf("expected error for %s", raw)
		}
	}
	if _, err := argvPyinfra("other", true, json.RawMessage(`{"roles":["common"]}`)); err == nil {
		t.Fatal("expected inventory error")
	}
}

func TestPyinfraRolesSorted(t *testing.T) {
	t.Parallel()
	if err := validatePyinfra("homelab", json.RawMessage(`{"roles":["bind","nut_exporter"],"limit":"router_nodes"}`)); err != nil {
		t.Fatal(err)
	}
	if slices.Contains(pyinfraRoles, "router") {
		t.Fatal("router should stay out until WireGuard keys are in the Job")
	}
	if !slices.IsSorted(pyinfraRoles) {
		t.Fatalf("roles not sorted: %v", pyinfraRoles)
	}
}

func TestPyinfraSummaryAndHostLimit(t *testing.T) {
	t.Parallel()
	params := json.RawMessage(`{"roles":["common","k8s"],"limit":"compute1.den1.kcloud.zone"}`)
	if err := validatePyinfra("homelab", params); err != nil {
		t.Fatal(err)
	}
	if got := summaryPyinfra(params); got != "common,k8s @ compute1.den1.kcloud.zone" {
		t.Fatalf("summary %q", got)
	}
}
