package argo

import (
	"context"
	"fmt"
	"net/url"
	"sync"
)

// Fake is an in-memory Argo client for tests.
type Fake struct {
	mu        sync.Mutex
	Apps      map[string]Status
	Synced    []string
	UIBase    string
	GetCalls  int
	ListCalls int
}

func NewFake() *Fake {
	return &Fake{Apps: map[string]Status{}}
}

func (f *Fake) Get(_ context.Context, app string) (Status, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.GetCalls++
	st, ok := f.Apps[app]
	if !ok {
		return Status{}, fmt.Errorf("app %s not found", app)
	}
	return cloneStatus(st), nil
}

func (f *Fake) List(_ context.Context) ([]Status, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ListCalls++
	out := make([]Status, 0, len(f.Apps))
	for name, st := range f.Apps {
		st.Name = name
		out = append(out, cloneStatus(st))
	}
	return out, nil
}

func (f *Fake) Sync(_ context.Context, app string, _ bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	st, ok := f.Apps[app]
	if !ok {
		return fmt.Errorf("app %s not found", app)
	}
	st.Sync = "Synced"
	st.Health = "Healthy"
	f.Apps[app] = st
	f.Synced = append(f.Synced, app)
	return nil
}

func (f *Fake) Calls() (get, list int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.GetCalls, f.ListCalls
}

func (f *Fake) Set(app string, st Status) {
	f.mu.Lock()
	defer f.mu.Unlock()
	st.Name = app
	f.Apps[app] = st
}

func cloneStatus(st Status) Status {
	if st.DeployedAt == nil {
		return st
	}
	t := st.DeployedAt.UTC()
	st.DeployedAt = &t
	return st
}

func (f *Fake) AppURL(app string) string {
	if f == nil || f.UIBase == "" || app == "" {
		return ""
	}
	u, err := url.JoinPath(f.UIBase, "applications", app)
	if err != nil {
		return ""
	}
	return u
}
