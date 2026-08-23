# deploybot

Small-scale release control plane: a deployable spec in git becomes a
Deployment and Argo Application (plus Service / HTTPRoute when the app is
routed), then you pin an image digest and promote homelab → prod.

Customers today: kmc console (`examples/kmc.yaml`), kmc-controller
(`examples/kmc-controller.yaml`), deploybot itself (`examples/deploybot.yaml`
+ `examples/deploybot-web.yaml`), and Play apps (`examples/sonarr.yaml`,
`radarr`, `bazarr`, `jackett`, `tautulli`, `ombi`, `flaresolverr`, `nzbget`,
`transmission`, `plex-exporter`, `plex`, `teamspeak`). Deploybot is self-hosted
in homelab and prod.
Deploybot generates the skeleton and the image pin. ConfigMaps, secrets, CRDs,
RBAC, and extra pod fields (env, volumes, args, securityContext, CIDRs) stay
as extra files / overlay patches.

Original intent and non-goals: [docs/goals.md](docs/goals.md).

## Layout

| Path | Role |
|------|------|
| `internal/` | Go: spec, render, pin, git write, Argo, HTTP API |
| `web/` | React Router 8 + Mantine console |
| `examples/` | Deployable specs and overlay patches |
| `deploybot.yaml` | Process config (per-cluster Argo / Headlamp / Grafana URLs, kube contexts). GitHub tokens stay in env |
| `Dockerfile` | API image: `ghcr.io/ianunruh/deploybot` |
| `web/Dockerfile` | Console image: `ghcr.io/ianunruh/deploybot-web` |

## Develop

```bash
just web-install
just check
just dev          # API :8080 + console :5173
```

CLI (dry-run unless `--apply`; nothing is pushed unless `--push`):

```bash
just build
./build/deploybot render examples/kmc.yaml --out /tmp/kmc-out
./build/deploybot pin --spec examples/kmc.yaml --stage homelab \
  --image ghcr.io/ianunruh/kmc@sha256:… --repo /path/to/kcloud-ops --apply --push
./build/deploybot promote --spec examples/kmc.yaml --from homelab --to prod \
  --repo /path/to/kcloud-ops --apply --push
./build/deploybot reconcile --spec examples/kmc.yaml --stage homelab \
  --repo /path/to/kcloud-ops --apply --push
```

`--apply` commits locally. `--push` (requires `--apply`) pushes the current
branch; it never force-pushes. `--sync` talks to Argo. Default is dry-run. For
`serve`, set `DEPLOYBOT_APPLY=1`, `DEPLOYBOT_PUSH=1`, and `DEPLOYBOT_SYNC=1`.

Process config is YAML (`deploybot.yaml` or `--config` / `DEPLOYBOT_CONFIG`).
Flags override env, env overrides the file. Clusters live in the file:

```yaml
clusters:
  homelab:
    argo:
      url: https://argocd.k8s.kcloud.zone
    headlamp:
      url: https://headlamp.k8s.kcloud.zone
    grafana:
      url: https://grafana.k8s.kcloud.zone
      logs: true
  prod:
    argo:
      url: https://argocd.k8s.kcloud.io
      kubeContext: prod-sjc1
    headlamp:
      url: https://headlamp.k8s.kcloud.io
    grafana:
      url: https://grafana.k8s.kcloud.io
```

Cluster names match promotion stages. `argo.url` is the Argo CD UI. Status
and sync talk to Application CRs through kubeconfig (`KUBECONFIG` or
`~/.kube/config`) — there is no Argo API token. `kubeContext` defaults to the
cluster name; `namespace` defaults to `argocd`. Optional `kubeconfig:` points
at a specific file. `DEPLOYBOT_ARGO_URL_<CLUSTER>` overrides a cluster UI URL.
Headlamp and Grafana `url` values are UI origins; observability links append
paths and query params. Grafana `logs: true` adds the Loki namespace
drilldown (homelab only today).

The pin picker lists published tags (newest first). `ghcr.io/…` uses the
GitHub Packages API; `docker.io/…` (including `lscr.io/…` and unprefixed
Hub names, rewritten to `docker.io`) uses the Docker Hub tags API. GitHub
auth is `DEPLOYBOT_GITHUB_TOKEN`, `GH_TOKEN`, `GITHUB_TOKEN`, or
`gh auth token`. The token needs `read:packages` (or `write:packages`).
Without that scope GHCR falls back to `main-<sha>` tags from git history.
Docker Hub listing is unauthenticated for public images; set
`DEPLOYBOT_DOCKERHUB_TOKEN` only if Hub rate-limits the API.

HTTPS git push uses `DEPLOYBOT_GIT_TOKEN`, then the same GitHub tokens as
the pin picker. SSH remotes use the ssh-agent (or `~/.ssh/id_ed25519` /
`id_rsa`).

Lint Go with golangci-lint v2.13+ built with Go 1.27 (Homebrew 2.10 is too old
for this toolchain):

```bash
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.1
```

## Hosted

Homelab and prod run the same specs in `deploybot-system` (`kcloud-ops`).

| Surface | Homelab | Prod |
|---------|---------|------|
| Console | `https://deploy.k8s.kcloud.zone` (internal Envoy) | `https://deploy.k8s.kcloud.io` (internal Envoy) |
| API | ClusterIP (`http://deploybot:8080`) | `https://deploy-api.k8s.kcloud.io` (external Envoy, GitHub OIDC) |

The console talks to the API in-cluster. GitHub Actions pins homelab through
the prod API. Promote to prod is a console action.

## Images

Push to `main` runs GitHub Actions: `CI` (Go test + golangci-lint, web `pnpm check`) and `Build and Push Docker Images` (same tags as kmc). After both images push, `pin-homelab` calls the prod API with GitHub OIDC.

| Image | Role |
|-------|------|
| `ghcr.io/ianunruh/deploybot` | Go API (`serve --addr :8080`, `/healthz`). Specs are baked at `/specs` (`examples/*.yaml`). Set `DEPLOYBOT_OPS_REPO_URL` to clone the ops repo into `DEPLOYBOT_OPS_REPO` at startup. |
| `ghcr.io/ianunruh/deploybot-web` | React Router console (`PORT=3000`). Talks to the API via `DEPLOYBOT_API_URL`. |

```bash
just docker
```
