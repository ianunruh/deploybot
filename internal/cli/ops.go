package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/ianunruh/deploybot/internal/argo"
	"github.com/ianunruh/deploybot/internal/config"
	"github.com/ianunruh/deploybot/internal/kube"
	"github.com/ianunruh/deploybot/internal/ops"
)

func runOps(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("ops requires catalog, ls, run, or logs")
	}
	switch args[0] {
	case "catalog":
		return runOpsCatalog(args[1:])
	case "ls", "list":
		return runOpsList(ctx, args[1:])
	case "run":
		return runOpsRun(ctx, args[1:])
	case "logs":
		return runOpsLogs(ctx, args[1:])
	default:
		return fmt.Errorf("unknown ops command %q", args[0])
	}
}

func runOpsCatalog(args []string) error {
	fs := flag.NewFlagSet("ops catalog", flag.ContinueOnError)
	cfgPath := configFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	svc, err := opsService(*cfgPath)
	if err != nil {
		return err
	}
	cat := svc.Catalog()
	for _, k := range cat.Kinds {
		fmt.Printf("%s\t%s\n", k.Name, k.Title)
		for _, f := range k.Fields {
			fmt.Printf("  --param %s= (%s)\n", f.Name, f.Type)
		}
	}
	if len(cat.Clusters) > 0 {
		fmt.Println("clusters", strings.Join(cat.Clusters, ", "))
	}
	return nil
}

func runOpsList(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("ops ls", flag.ContinueOnError)
	cfgPath := configFlag(fs)
	kind := fs.String("kind", "", "filter by kind")
	cluster := fs.String("cluster", "", "filter by cluster")
	if err := fs.Parse(args); err != nil {
		return err
	}
	svc, err := opsService(*cfgPath)
	if err != nil {
		return err
	}
	list, err := svc.List(ctx, *kind, *cluster)
	if err != nil {
		return err
	}
	if len(list) == 0 {
		fmt.Println("no executions")
		return nil
	}
	for _, ex := range list {
		dry := ""
		if ex.DryRun {
			dry = " dry-run"
		}
		fmt.Printf("%s\t%s\t%s\t%s\t%s%s\n", ex.ID, ex.Cluster, ex.Kind, ex.Phase, ex.Summary, dry)
	}
	return nil
}

func runOpsRun(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("ops run", flag.ContinueOnError)
	cfgPath := configFlag(fs)
	kind := fs.String("kind", "", "ops kind (required)")
	cluster := fs.String("cluster", "", "target cluster (required)")
	ref := fs.String("ref", "", "git ref to clone (default from config)")
	apply := fs.Bool("apply", false, "run for real (default is dry-run)")
	paramsJSON := fs.String("params", "", "kind params as JSON")
	var params paramList
	fs.Var(&params, "param", "kind param key=value (repeatable; comma-separated values become arrays)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *kind == "" || *cluster == "" {
		return fmt.Errorf("ops run requires --kind and --cluster")
	}
	raw, err := mergeParams(*paramsJSON, params)
	if err != nil {
		return err
	}
	svc, err := opsService(*cfgPath)
	if err != nil {
		return err
	}
	dry := !*apply
	ex, err := svc.Start(ctx, ops.Request{
		Kind:    *kind,
		Cluster: *cluster,
		DryRun:  &dry,
		Ref:     *ref,
		Params:  raw,
	})
	if err != nil {
		return err
	}
	if ex.DryRun {
		fmt.Println("dry-run (pass --apply to run for real)")
	}
	fmt.Printf("started %s on %s (%s)\n", ex.ID, ex.Cluster, ex.Summary)
	return nil
}

func runOpsLogs(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("ops logs", flag.ContinueOnError)
	cfgPath := configFlag(fs)
	cluster := fs.String("cluster", "", "cluster (required)")
	follow := fs.Bool("follow", false, "stream until the pod exits")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *cluster == "" || fs.NArg() != 1 {
		return fmt.Errorf("ops logs requires --cluster and an execution id")
	}
	svc, err := opsService(*cfgPath)
	if err != nil {
		return err
	}
	rc, err := svc.Logs(ctx, *cluster, fs.Arg(0), *follow)
	if err != nil {
		return err
	}
	defer func() { _ = rc.Close() }()
	_, err = io.Copy(os.Stdout, rc)
	return err
}

func opsService(cfgPath string) (*ops.Service, error) {
	file, _, err := config.Open(cfgPath)
	if err != nil {
		return nil, err
	}
	eps, err := argo.EndpointsFromConfig(file.Clusters)
	if err != nil {
		return nil, err
	}
	return &ops.Service{
		Config: ops.ConfigFromFile(file),
		Kube:   kubeClients(eps),
		Names:  clusterNameList(file),
	}, nil
}

func kubeClients(eps argo.Endpoints) map[string]*kube.REST {
	out := map[string]*kube.REST{}
	for name, ep := range eps {
		if r := argo.REST(ep); r != nil {
			out[name] = r
		}
	}
	return out
}

func clusterNameList(file *config.File) []string {
	if file == nil {
		return nil
	}
	names := make([]string, 0, len(file.Clusters))
	for name := range file.Clusters {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

type paramList []string

func (p *paramList) String() string { return strings.Join(*p, ",") }

func (p *paramList) Set(v string) error {
	if !strings.Contains(v, "=") {
		return fmt.Errorf("param %q must be key=value", v)
	}
	*p = append(*p, v)
	return nil
}

func mergeParams(rawJSON string, pairs paramList) (json.RawMessage, error) {
	top := map[string]any{}
	if strings.TrimSpace(rawJSON) != "" {
		if err := json.Unmarshal([]byte(rawJSON), &top); err != nil {
			return nil, fmt.Errorf("--params: %w", err)
		}
	}
	data, _ := top["data"].(map[string]any)
	for _, p := range pairs {
		k, v, _ := strings.Cut(p, "=")
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if rest, ok := strings.CutPrefix(k, "data."); ok {
			if data == nil {
				data = map[string]any{}
				top["data"] = data
			}
			data[rest] = v
			continue
		}
		if strings.Contains(v, ",") {
			top[k] = splitCSV(v)
		} else {
			top[k] = v
		}
	}
	if len(top) == 0 {
		return json.RawMessage(`{}`), nil
	}
	return json.Marshal(top)
}

func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
