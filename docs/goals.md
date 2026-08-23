# Goals

Started at the end of the first build session (2026-08-22) so later work
does not have to reconstruct this from chat. Updated after kmc console and
kmc-controller were cut over.

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
  jackett, …) is first-class later; it is the simpler workload, not a
  different product.
- **Do not grow the spec into a Deployment schema.** Generate the skeleton
  (Deployment, optional Service/HTTPRoute, overlay `images:`, Argo app).
  App-specific fields (env, volumes, args, securityContext, extra ports,
  CRDs, RBAC, PVCs) stay as extra files and overlay patches. Sync rewrites
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
`kcloud-ops`. `patch-web.yaml` (envFrom, env, clusters volume) and prod
`patch-cluster-tokens.yaml` are overlay-owned, plus `clusters.yaml`, env
ConfigMaps, `gen-secrets.sh`, impersonator SA/RBAC.

**kmc-controller** (`examples/kmc-controller.yaml`): no route. Generated
Deployment + Argo apps only. CRDs and RBAC copied from
`kmc/deploy/controller`. `patch-manager.yaml` (securityContext, extra ports)
and per-stage `patch-cidrs.yaml` (cluster CIDR args) are overlay-owned.

GitHub Actions keeps building; deploybot consumes the image. Do not move
Kaniko or rewrite `docker-build.yml` for this.

## What this repo already did

Spec + renderer + goldens for both customers, image pin (GHCR picker, newest
first), local git write, opt-in git push (no force), Argo sync/health/promote,
RR console (catalog, stage matrix, pin, promote, per-stage sync).

Pin/promote **only** upsert `images:` on the stage overlay. They must not
rewrite workload YAML or shared Argo project `kustomization.yaml` files
(sandbox app list).

`deploybot sync` renders generated manifests into the ops repo (merge
kustomizations, keep pins and human generators/patches) and can limit stages.
Argo Applications are full docs per overlay, not a shared base + path patch.

Route, hostname/gateway, and containerPort are optional. Probes can set path
per probe without a shared `probes.path`.

## Explicitly not done yet

- Play / ivy as spec customers.
- Preview apps, multi-service releases, replacing Actions/Kaniko.

## Non-goals

- Replacing Argo or the image builder.
- A generic workflow engine or Backstage.
- Multi-tenant IAM (Dex + Tailscale is enough).
- Flattening cluster vs app env: homelab/prod are clusters; ivy delta/gamma
  are app stages on homelab. The spec should allow both.
- Modeling every Kubernetes field in the Deployable spec.
