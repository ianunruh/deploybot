package ops

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/ianunruh/deploybot/internal/kube"
)

var (
	ErrNoImage        = errors.New("ops image is not configured")
	ErrUnknownCluster = errors.New("unknown cluster")
	ErrClusterOffline = errors.New("cluster kube is not configured")
	ErrBusy           = errors.New("an ops execution is already running on this cluster")
	ErrNotFound       = errors.New("unknown execution")
)

// Service creates and lists ops Jobs on per-cluster kube clients.
type Service struct {
	Config Config
	Kube   map[string]*kube.REST
	Names  []string
	Actor  Actor
}

func (s *Service) WithActor(a Actor) *Service {
	if s == nil {
		return nil
	}
	out := *s
	out.Actor = a
	return &out
}

func (s *Service) clusterNames() []string {
	if s == nil {
		return nil
	}
	names := slices.Clone(s.Names)
	if len(names) == 0 {
		for name := range s.Kube {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	return names
}

func (s *Service) rest(cluster string) (*kube.REST, error) {
	cluster = strings.ToLower(strings.TrimSpace(cluster))
	if cluster == "" {
		return nil, fmt.Errorf("cluster is required")
	}
	if s == nil {
		return nil, ErrClusterOffline
	}
	known := false
	for _, name := range s.clusterNames() {
		if name == cluster {
			known = true
			break
		}
	}
	if !known && s.Kube[cluster] == nil {
		return nil, fmt.Errorf("%w: %s", ErrUnknownCluster, cluster)
	}
	c := s.Kube[cluster]
	if c == nil {
		return nil, fmt.Errorf("%w: %s", ErrClusterOffline, cluster)
	}
	return c, nil
}

// Start validates the request, builds argv from the Kind, and creates a Job.
func (s *Service) Start(ctx context.Context, req Request) (Execution, error) {
	if s == nil {
		return Execution{}, ErrClusterOffline
	}
	if strings.TrimSpace(s.Config.Image) == "" {
		return Execution{}, ErrNoImage
	}
	kindName := strings.TrimSpace(req.Kind)
	k := Lookup(kindName)
	if k == nil {
		return Execution{}, unknownKind(kindName)
	}
	cluster := strings.ToLower(strings.TrimSpace(req.Cluster))
	rest, err := s.rest(cluster)
	if err != nil {
		return Execution{}, err
	}
	if err := k.Validate(cluster, req.Params); err != nil {
		return Execution{}, err
	}
	argv, err := k.Argv(cluster, req.dryRun(), req.Params)
	if err != nil {
		return Execution{}, err
	}
	if err := s.guardBusy(ctx, rest, cluster); err != nil {
		return Execution{}, err
	}
	job := buildJob(s.Config, cluster, *k, req, argv, s.Actor)
	var created batchJob
	if err := rest.Post(ctx, jobsPath(s.Config.ns()), job, &created); err != nil {
		return Execution{}, fmt.Errorf("create job: %w", err)
	}
	return executionFromJob(cluster, created, ""), nil
}

func (s *Service) guardBusy(ctx context.Context, rest *kube.REST, cluster string) error {
	jobs, err := listJobs(ctx, rest, s.Config.ns(), executionSelector("", cluster))
	if err != nil {
		return err
	}
	for _, j := range jobs {
		if jobActive(j) {
			return fmt.Errorf("%w (%s %s)", ErrBusy, j.Metadata.Name, jobPhase(j))
		}
	}
	return nil
}

// List returns executions, newest first. kind/cluster empty means all.
func (s *Service) List(ctx context.Context, kind, cluster string) ([]Execution, error) {
	if s == nil {
		return nil, ErrClusterOffline
	}
	clusters := s.clusterNames()
	if cluster != "" {
		clusters = []string{strings.ToLower(strings.TrimSpace(cluster))}
	}
	var out []Execution
	var first error
	var listed bool
	for _, name := range clusters {
		rest, err := s.rest(name)
		if err != nil {
			if first == nil {
				first = err
			}
			continue
		}
		jobs, err := listJobs(ctx, rest, s.Config.ns(), executionSelector(kind, name))
		if err != nil {
			if first == nil {
				first = err
			}
			continue
		}
		listed = true
		for _, j := range jobs {
			podName, _ := currentPod(ctx, rest, s.Config.ns(), j.Metadata.Name)
			out = append(out, executionFromJob(name, j, podName))
		}
	}
	if !listed && first != nil && cluster != "" {
		return nil, first
	}
	slices.SortFunc(out, func(a, b Execution) int {
		at, bt := a.CreatedAt, b.CreatedAt
		switch {
		case at == nil && bt == nil:
			return strings.Compare(b.ID, a.ID)
		case at == nil:
			return 1
		case bt == nil:
			return -1
		case bt.After(*at):
			return 1
		case at.After(*bt):
			return -1
		default:
			return strings.Compare(b.ID, a.ID)
		}
	})
	return out, nil
}

// Get returns one execution. cluster is required.
func (s *Service) Get(ctx context.Context, cluster, id string) (Execution, error) {
	if strings.TrimSpace(id) == "" {
		return Execution{}, ErrNotFound
	}
	rest, err := s.rest(cluster)
	if err != nil {
		return Execution{}, err
	}
	var j batchJob
	if err := rest.Get(ctx, jobPath(s.Config.ns(), id), &j); err != nil {
		var se *kube.StatusError
		if errors.As(err, &se) && se.Code == http.StatusNotFound {
			return Execution{}, fmt.Errorf("%w: %s", ErrNotFound, id)
		}
		return Execution{}, err
	}
	if j.Metadata.Labels[labelExecution] != "true" {
		return Execution{}, fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	podName, _ := currentPod(ctx, rest, s.Config.ns(), j.Metadata.Name)
	return executionFromJob(cluster, j, podName), nil
}

func listJobs(ctx context.Context, rest *kube.REST, ns, selector string) ([]batchJob, error) {
	q := url.Values{"labelSelector": {selector}}
	var list jobList
	if err := rest.Get(ctx, jobsPath(ns)+"?"+q.Encode(), &list); err != nil {
		return nil, err
	}
	return list.Items, nil
}

func currentPod(ctx context.Context, rest *kube.REST, ns, jobName string) (string, error) {
	q := url.Values{"labelSelector": {"job-name=" + jobName}}
	var list podList
	if err := rest.Get(ctx, podsPath(ns)+"?"+q.Encode(), &list); err != nil {
		return "", err
	}
	if len(list.Items) == 0 {
		return "", nil
	}
	best := list.Items[0]
	for _, p := range list.Items[1:] {
		if p.Metadata.CreationTimestamp != nil && (best.Metadata.CreationTimestamp == nil || p.Metadata.CreationTimestamp.After(*best.Metadata.CreationTimestamp)) {
			best = p
		}
	}
	return best.Metadata.Name, nil
}

func (s *Service) waitPod(ctx context.Context, rest *kube.REST, ns, jobName string) (string, error) {
	tick := time.NewTicker(500 * time.Millisecond)
	defer tick.Stop()
	for {
		name, err := currentPod(ctx, rest, ns, jobName)
		if err != nil {
			return "", err
		}
		if name != "" {
			return name, nil
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-tick.C:
		}
	}
}
