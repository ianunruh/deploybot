# deploybot

**Easy staged deploys across Kubernetes clusters.**

A Deployable spec in git becomes a Kubernetes workload and Argo CD
Application (plus Service / HTTPRoute when the app is routed). From there
you pin an image digest and promote it across stages — typically
homelab → prod. Git remains the source of truth: deploybot commits the
pin, optionally pushes, and asks Argo to sync. It does not `kubectl apply`
your app.

## What you get

- **Specs that stay small.** One YAML file names the image, workload,
  optional HTTPRoute, and stages. ConfigMaps, secrets, CRDs, RBAC, and
  extra pod fields (env, volumes, args, securityContext, CIDRs) stay as
  extra files / overlay patches.
- **Generated skeleton.** Deployment or StatefulSet, Service, HTTPRoute,
  overlay `images:`, and an Argo Application per stage. `reconcile`
  rewrites generated paths and merges kustomizations so those extras
  survive.
- **Digest pins.** Pick a published tag from GHCR or Docker Hub (newest
  first). The pin is a kustomize `images:` entry on the stage overlay —
  pin and promote never rewrite workload YAML.
- **Staged promotion.** Copy a pin from one stage to the next. Gates for
  Argo health, bake time, and human approval. GitHub Actions can pin
  homelab through the API; promote to prod is a console action.
- **Rollback.** Re-pin a digest that stage already had. History rows and
  a degraded-stage banner open the same commit / sync / rollout theater
  as pin. It is not `kubectl rollout undo`.
- **Console and CLI.** Catalog of deployables, per-app release flow,
  pin / promote / rollback with diffs, release history, and links out to
  Argo, Headlamp, and Grafana. The CLI is the same loop; dry-run unless
  `--apply`, nothing is pushed unless `--push`, and it never force-pushes.
- **GitOps, not a second source of truth.** Deploybot writes git and
  talks to Application CRs. Argo applies the cluster.

It runs itself in homelab and prod, plus the kmc console and controller
and Play apps (Sonarr, Radarr, Plex, Transmission, …). Specs are in
[`examples/`](examples/). Original intent and non-goals:
[docs/goals.md](docs/goals.md).

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

### CLI

Dry-run unless `--apply`; nothing is pushed unless `--push`:

```bash
just build
./build/deploybot render examples/kmc.yaml --out /tmp/kmc-out
./build/deploybot pin --spec examples/kmc.yaml --stage homelab \
  --image ghcr.io/ianunruh/kmc@sha256:… --repo /path/to/kcloud-ops --apply --push
./build/deploybot promote --spec examples/kmc.yaml --from homelab --to prod \
  --repo /path/to/kcloud-ops --apply --push
./build/deploybot rollback --spec examples/kmc.yaml --stage homelab \
  --image ghcr.io/ianunruh/kmc@sha256:… --repo /path/to/kcloud-ops --apply --push
./build/deploybot reconcile --spec examples/kmc.yaml --stage homelab \
  --repo /path/to/kcloud-ops --apply --push
```

`--apply` commits locally. `--push` (requires `--apply`) pushes the current
branch; it never force-pushes. `--sync` talks to Argo. Default is dry-run. For
`serve`, set `DEPLOYBOT_APPLY=1`, `DEPLOYBOT_PUSH=1`, and `DEPLOYBOT_SYNC=1`.
Scheduled auto-pin is off unless `DEPLOYBOT_AUTO_PIN=1` (or `--auto-pin` /
`autoPin: true`); enable it on exactly one serve instance so homelab and
prod do not race the same ops repo.

### Config

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

### Registry and git auth

The pin picker lists published tags (newest first). `ghcr.io/…` uses the
GitHub Packages API; `docker.io/…` (including `lscr.io/…` and unprefixed
Hub names, rewritten to `docker.io`) uses the Docker Hub tags API. GitHub
auth is `DEPLOYBOT_GITHUB_TOKEN`, `GH_TOKEN`, `GITHUB_TOKEN`, or
`gh auth token`. The token needs `read:packages` (or `write:packages`).
Without that scope GHCR falls back to `main-<sha>` tags from git history.
Docker Hub listing is unauthenticated for public images; set
`DEPLOYBOT_DOCKERHUB_TOKEN` only if Hub rate-limits the API.

Third-party images opt into registry tracking with `spec.update`. The
console **Updates** page compares the first-stage pin to the newest
published digest. `spec.update.match` is an optional Go regex that limits
which tags count as newest (linuxserver `v1.2.3-ls123` instead of the
floating `1.6.0` / `latest` aliases). `spec.update.auto: 24h` enrolls the
app; `serve` only writes those pins when auto-pin is enabled on that
process. Promote gates are unchanged — prod still needs approval.
`deploybot update` is the same check from the CLI (dry-run unless
`--apply`).

HTTPS git push uses `DEPLOYBOT_GIT_TOKEN`, then the same GitHub tokens as
the pin picker. SSH remotes use the ssh-agent (or `~/.ssh/id_ed25519` /
`id_rsa`).

### Lint

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
| `ghcr.io/ianunruh/deploybot-web` | React Router console (`PORT=3000`, `/healthz`). Talks to the API via `DEPLOYBOT_API_URL`. |

```bash
just docker
```
