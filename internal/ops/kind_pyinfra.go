package ops

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

const KindPyinfra = "pyinfra"

var hostName = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{0,253}$`)

var pyinfraRoles = []string{
	"bind",
	"bridge_vlan",
	"common",
	"containerd",
	"k8s",
	"k8s_vip",
	"nas_node_exporter",
	"nic_tuning",
	"node_exporter",
	"nut_exporter",
	"nvidia_driver",
	"proxmox_template",
	"scrutiny",
}

var pyinfraLimits = []string{
	"bridge_vlan_nodes",
	"exporter_nodes",
	"gpu_nodes",
	"k8s_nodes",
	"k8s_vip_nodes",
	"maas_nodes",
	"nas_nodes",
	"nic_tuning_nodes",
	"proxmox_nodes",
	"router_nodes",
	"scrutiny_nodes",
}

var pyinfraDataKeys = []string{
	"k8s_package_set",
	"k8s_discovery_token",
	"k8s_control_plane_certificate_key",
	"k8s_join_source_host",
	"ubuntu_codename",
	"ubuntu_arch",
	"template_date",
	"proxmox_storage",
}

var pyinfraPackageSets = []string{"all", "kubeadm", "kubelet"}

func init() {
	register(Kind{
		Name:    KindPyinfra,
		Title:   "pyinfra",
		WorkDir: "deploys",
		Fields: []Field{
			{
				Name:     "roles",
				Type:     FieldMulti,
				Title:    "Roles",
				Required: true,
				Options:  pyinfraRoles,
			},
			{
				Name:        "limit",
				Type:        FieldString,
				Title:       "Limit",
				Description: "Inventory group or hostname.",
				Required:    true,
				Suggestions: pyinfraLimits,
			},
			{
				Name:  "data",
				Type:  FieldMap,
				Title: "Extra --data",
				Keys:  pyinfraDataKeys,
			},
		},
		Validate: validatePyinfra,
		Argv:     argvPyinfra,
		Summary:  summaryPyinfra,
	})
}

type pyinfraParams struct {
	Roles []string
	Limit string
	Data  map[string]string
}

func parsePyinfra(params json.RawMessage) (pyinfraParams, error) {
	raw := map[string]any{}
	if len(params) > 0 && string(params) != "null" {
		if err := json.Unmarshal(params, &raw); err != nil {
			return pyinfraParams{}, fmt.Errorf("pyinfra params: %w", err)
		}
	}
	if err := requireKeys(raw, "roles", "limit", "data"); err != nil {
		return pyinfraParams{}, fmt.Errorf("pyinfra: %w", err)
	}
	out := pyinfraParams{
		Roles: stringSlice(raw["roles"]),
		Limit: strings.TrimSpace(asString(raw["limit"])),
	}
	if raw["data"] != nil {
		data, err := stringMap(raw["data"])
		if err != nil {
			return pyinfraParams{}, fmt.Errorf("pyinfra data: %w", err)
		}
		out.Data = data
	}
	return out, nil
}

func validatePyinfra(_ string, params json.RawMessage) error {
	p, err := parsePyinfra(params)
	if err != nil {
		return err
	}
	if len(p.Roles) == 0 {
		return fmt.Errorf("pyinfra: roles is required")
	}
	seen := map[string]bool{}
	for _, role := range p.Roles {
		if !slices.Contains(pyinfraRoles, role) {
			return fmt.Errorf("pyinfra: unknown role %q", role)
		}
		if seen[role] {
			return fmt.Errorf("pyinfra: duplicate role %q", role)
		}
		seen[role] = true
	}
	if p.Limit == "" {
		return fmt.Errorf("pyinfra: limit is required")
	}
	if !slices.Contains(pyinfraLimits, p.Limit) && !hostName.MatchString(p.Limit) {
		return fmt.Errorf("pyinfra: invalid limit %q", p.Limit)
	}
	for k, v := range p.Data {
		if !slices.Contains(pyinfraDataKeys, k) {
			return fmt.Errorf("pyinfra: unknown data key %q", k)
		}
		if strings.TrimSpace(v) == "" {
			return fmt.Errorf("pyinfra: empty data %q", k)
		}
		if k == "k8s_package_set" && !slices.Contains(pyinfraPackageSets, v) {
			return fmt.Errorf("pyinfra: k8s_package_set must be all, kubeadm, or kubelet")
		}
	}
	return nil
}

func argvPyinfra(cluster string, dryRun bool, params json.RawMessage) ([]string, error) {
	p, err := parsePyinfra(params)
	if err != nil {
		return nil, err
	}
	inv, err := pyinfraInventory(cluster)
	if err != nil {
		return nil, err
	}
	argv := []string{"pyinfra", inv}
	for _, role := range p.Roles {
		argv = append(argv, role+"/deploy.py")
	}
	if p.Limit != "" {
		argv = append(argv, "--limit", p.Limit)
	}
	if dryRun {
		argv = append(argv, "--dry")
	}
	argv = append(argv, "-y")
	keys := make([]string, 0, len(p.Data))
	for k := range p.Data {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		argv = append(argv, "--data", k+"="+p.Data[k])
	}
	return argv, nil
}

func summaryPyinfra(params json.RawMessage) string {
	p, err := parsePyinfra(params)
	if err != nil {
		return ""
	}
	roles := strings.Join(p.Roles, ",")
	if p.Limit == "" {
		return roles
	}
	return roles + " @ " + p.Limit
}

func pyinfraInventory(cluster string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(cluster)) {
	case "homelab":
		return "inventory.py", nil
	case "prod":
		return "inventory-prod.py", nil
	default:
		return "", fmt.Errorf("pyinfra: no inventory for cluster %q", cluster)
	}
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}

func stringMap(v any) (map[string]string, error) {
	switch t := v.(type) {
	case nil:
		return nil, nil
	case map[string]string:
		return t, nil
	case map[string]any:
		out := make(map[string]string, len(t))
		for k, item := range t {
			s, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("%s: want string", k)
			}
			out[k] = s
		}
		return out, nil
	default:
		return nil, fmt.Errorf("want object")
	}
}
