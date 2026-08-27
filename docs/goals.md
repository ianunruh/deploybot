# Goals

Started at the end of the first build session (2026-08-22) so later work
does not have to reconstruct this from chat. Updated after deploybot itself
was cut over in homelab and prod (2026-08-23).

## Why

Homelab services follow a repeated pattern: git definition, k8s workload +
HTTPRoute, Argo CD apply, staged rollout. Today that is either hand-applied
YAML or a copy-pasted GitLab job (see ivy-universe: Kaniko build, bump a tag
in ivy-ops, `argocd app sync`, delta then gamma).

The work-inspired shape:

1. Define a **deployable** in git.
2. Get **scaffolded k8s** (Deployment, optional Service/HTTPRoute, Argo
   Application). Extra pod and cluster bits live in overlay patches.
3. On merge to the default branch, apply via **Argo**, staged (homelab then
   production; ivy-style delta → gamma is the same idea with different names).
4. A **web UI** to see what is where and to pin / promote by hand.

This is a small-scale control plane on top of GitLab/GitHub + Argo + Gateway
API, not a replacement for them.

## Product shape

- **Git remains source of truth.** Deploybot writes git and asks Argo to sync.
  It does not `kubectl apply` app workloads. `--apply` commits locally;
  `--push` updates the remote (never force-pushes).
- **Dashboard is not enough.** A UI over GitLab + Argo does not kill the
  copy-paste CI. Deploybot owns the spec → manifests → pin → health-gated
  promote loop.
- **Two app flavors, one spec:** source-built (image from CI SHA) and
  third-party (pinned upstream tag, often a StatefulSet). Play (sonarr,
  radarr, bazarr, jackett, tautulli, ombi) uses the same spec; `lscr.io`
  canonicalizes to `docker.io`.
- **Do not grow the spec into a Deployment schema.** Generate the skeleton
  (Deployment, optional Service/HTTPRoute, overlay `images:`, Argo app).
  App-specific fields (env, volumes, args, securityContext, extra ports,
  CRDs, RBAC, PVCs) stay as extra files and overlay patches. Reconcile rewrites
  generated paths and merges kustomizations so those extras survive.

## Stack

- Console: React Router 8 framework mode + Mantine (same as kmc / ivy-web).
- Orchestration: Go (render, git commit, Argo wait, promote). Not a React
  Router action — promotion is a minutes-long state machine.
- No Deployable CRD for v1. Git commit is the event; a controller is extra
  API surface.

## Customers

**kmc console** (`examples/kmc.yaml`): Deployment, Service, HTTPRoute, Argo
Application, homelab → prod, pin by digest/`main-<sha>`. Cut over in
`kcloud-ops`. GitHub Actions pins homelab through the prod API. `patch-web.yaml`
(envFrom, env, clusters volume) and prod `patch-cluster-tokens.yaml` are
overlay-owned, plus `clusters.yaml`, env ConfigMaps, `gen-secrets.sh`,
impersonator SA/RBAC.

**kmc-controller** (`examples/kmc-controller.yaml`): no route. Generated
Deployment + Argo apps only. Same kmc repo CI pins it with the console. CRDs
and RBAC copied from `kmc/deploy/controller`. `patch-manager.yaml`
(securityContext, extra ports) and per-stage `patch-cidrs.yaml` (cluster CIDR
args) are overlay-owned.

**deploybot** (`examples/deploybot.yaml` + `examples/deploybot-web.yaml`):
self-hosted control plane in `deploybot-system`, live in homelab and prod.
API is ClusterIP in-cluster (console talks to it directly; Service, SA, Argo
RBAC, git clone of kcloud-ops, and kubeconfig are overlay-owned). Prod
exposes the API at `deploy-api.k8s.kcloud.io` on Gateway `external` / `https`
with a GitHub Actions OIDC SecurityPolicy (allow `ianunruh/deploybot`,
`ianunruh/kmc`, and `ianunruh/humpty` on `refs/heads/main`). Console is routed
at `deploy.k8s.kcloud.zone` (Gateway `internal` / `https`) and
`deploy.k8s.kcloud.io` (Gateway `external` / `https`) with edge-sso (Dex
`gateway-sso` OIDC). Specs are baked into the API image. GitHub Actions
builds both images and pins homelab through the prod API; promote to prod
is a console action.

GitHub Actions keeps building; deploybot consumes the image. Do not move
Kaniko or rewrite the image build in `docker-build.yml` for this.

**humpty** (`examples/humpty.yaml`): private crash-test dummy for pin,
promote, bad commits, and rollback. Routed Deployment in `deploybot-system`
(reuses `ghcr-auth`), sandbox Argo project, homelab → prod with approval.
GitHub Actions pins homelab through the prod API. No overlay patches.
`ghcr.io/ianunruh/humpty:broken` crashloops on start; pin it, then roll
back to the previous homelab digest from the console.

**Play** (`examples/sonarr.yaml`, `radarr`, `bazarr`, `jackett`, `tautulli`,
`ombi`, `flaresolverr`, `nzbget`, `transmission`, `plex-exporter`, `plex`,
`teamspeak`): mix of linuxserver StatefulSets, a GHCR Deployment
(flaresolverr), haugene transmission-openvpn, plex-exporter (no route),
Plex (LoadBalancer, no HTTPRoute), and teamspeak (namespace `teamspeak`,
UDP/TCP LoadBalancers). App-specific env, volumes, extra Services, PVCs,
certs, Infisical, and HTTPRoute filters stay as overlay patches / extra
files. `lscr.io` and unprefixed Hub names canonicalize to `docker.io`.

## What this repo already did

Spec + renderer + goldens for kmc, kmc-controller, deploybot itself, and
Play apps (sonarr, radarr, bazarr, jackett, tautulli, ombi, flaresolverr,
nzbget, transmission, plex-exporter, plex, teamspeak). Image pin (GHCR or
Docker Hub picker, newest first; `lscr.io` canonicalizes to `docker.io`),
registry tracking for third-party images (`spec.update`, optional
`auto: 24h` first-stage pin; `serve` auto-pin is a process flag so only
one instance writes), local git write, opt-in git push (no force),
Argo sync/health/promote, rollback to a previous overlay pin (console
history + degraded banner; same git write as pin), RR console (catalog,
updates, stage matrix, pin, promote, rollback, per-stage reconcile).
Deploybot is cut over in homelab and prod; push to `main` pins homelab via
the prod API (GitHub OIDC).

Pin/promote **only** upsert `images:` on the stage overlay. They must not
rewrite workload YAML or shared Argo project `kustomization.yaml` files
(sandbox app list).

`deploybot reconcile` renders generated manifests into the ops repo (merge
kustomizations, keep pins and human generators/patches) and can limit stages.
Argo Applications are full docs per overlay, not a shared base + path patch.

Route, hostname/gateway, and containerPort are optional. Probes can set path
per probe without a shared `probes.path`.

## Explicitly not done yet

- Remaining Play apps in kcloud-ops (plexportal, play-common).
- ivy as a spec customer.
- Preview apps, multi-service releases, replacing Actions/Kaniko.

## Non-goals

- Replacing Argo or the image builder.
- A generic workflow engine or Backstage.
- Multi-tenant IAM (Dex + Tailscale is enough).
- Flattening cluster vs app env: homelab/prod are clusters; ivy delta/gamma
  are app stages on homelab. The spec should allow both.
- Modeling every Kubernetes field in the Deployable spec.
