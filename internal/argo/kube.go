package argo

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"github.com/ianunruh/deploybot/internal/config"
	"github.com/ianunruh/deploybot/internal/kube"
)

const defaultNamespace = "argocd"

// KubeClient implements Client against Application CRs (argocd --core).
type KubeClient struct {
	REST      *kube.REST
	Namespace string
	UIBaseURL string
}

func (c *KubeClient) appPath(app string) string {
	ns := c.Namespace
	if ns == "" {
		ns = defaultNamespace
	}
	return "/apis/argoproj.io/v1alpha1/namespaces/" + url.PathEscape(ns) + "/applications/" + url.PathEscape(app)
}

func (c *KubeClient) Get(ctx context.Context, app string) (Status, error) {
	var raw argoApp
	if err := c.REST.Get(ctx, c.appPath(app), &raw); err != nil {
		return Status{}, fmt.Errorf("argo kube get %s: %w", app, err)
	}
	return Status{
		Name:     raw.Metadata.Name,
		Health:   raw.Status.Health.Status,
		Sync:     raw.Status.Sync.Status,
		Revision: raw.Status.Sync.Revision,
		Message:  raw.Status.Health.Message,
	}, nil
}

func (c *KubeClient) Sync(ctx context.Context, app string, prune bool) error {
	patch := map[string]any{
		"operation": map[string]any{
			"initiatedBy": map[string]any{"username": "deploybot"},
			"sync":        map[string]any{"prune": prune},
		},
	}
	if err := c.REST.Patch(ctx, c.appPath(app), kube.MergePatch, patch); err != nil {
		return fmt.Errorf("argo kube sync %s: %w", app, err)
	}
	return nil
}

func (c *KubeClient) AppURL(app string) string {
	return joinAppURL(c.UIBaseURL, app)
}

func kubeClientFor(st config.Argo, stage, uiURL string) (*KubeClient, error) {
	contextName := st.KubeContext
	if contextName == "" {
		contextName = stage
	}
	path := kube.ResolvePath(st.Kubeconfig)
	rest, err := kube.LoadREST(path, contextName)
	if err != nil {
		if errors.Is(err, kube.ErrNoConfig) && st.Kubeconfig == "" {
			return nil, nil
		}
		if errors.Is(err, kube.ErrNoContext) && st.KubeContext == "" {
			return nil, nil
		}
		return nil, fmt.Errorf("argo %s kube: %w", stage, err)
	}
	ns := st.Namespace
	if ns == "" {
		ns = defaultNamespace
	}
	return &KubeClient{REST: rest, Namespace: ns, UIBaseURL: uiURL}, nil
}
