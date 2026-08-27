package render

import (
	"strings"

	"github.com/ianunruh/deploybot/internal/spec"
	"github.com/ianunruh/deploybot/internal/yamlx"
)

func writeArgo(out Tree, d *spec.Deployable) error {
	for _, st := range d.Spec.Stages {
		app, err := yamlx.MarshalGenerated(applicationObj(d, st))
		if err != nil {
			return err
		}
		out[ApplicationOverlayPath(d, st.Name)] = app
	}
	return nil
}

func applicationObj(d *spec.Deployable, st spec.Stage) application {
	ann := map[string]string{
		"argocd.argoproj.io/manifest-generate-paths": manifestGeneratePaths(d),
	}
	app := application{
		APIVersion: "argoproj.io/v1alpha1",
		Kind:       "Application",
		Metadata: objectMeta{
			Name:        d.Spec.Argo.Name,
			Labels:      map[string]string{"argocd.argoproj.io/app-type": "kustomize"},
			Annotations: ann,
		},
		Spec: appSpec{
			Project: d.Spec.Argo.Project,
			Source: &appSource{
				Path:           overlayPath(d, st.Name),
				RepoURL:        d.Spec.Git.RepoURL,
				TargetRevision: d.Spec.Git.TargetRevision,
			},
			Destination: &appDest{
				Namespace: d.Spec.Namespace,
				Server:    d.Spec.Argo.DestinationServer,
			},
		},
	}
	if d.Spec.Argo.CreateNamespace {
		app.Spec.SyncPolicy = &syncPolicy{SyncOptions: []string{"CreateNamespace=true"}}
	}
	return app
}

func manifestGeneratePaths(d *spec.Deployable) string {
	paths := []string{"/" + strings.TrimPrefix(d.Spec.Git.WorkloadPath, "/")}
	seen := map[string]struct{}{paths[0]: {}}
	for _, p := range d.Spec.Argo.GeneratePaths {
		p = "/" + strings.TrimPrefix(strings.TrimSpace(p), "/")
		if p == "/" {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		paths = append(paths, p)
	}
	return strings.Join(paths, ";")
}
