# Original goals

Written at the end of the first build session (2026-08-22) so later work
does not have to reconstruct this from chat.

## Why

Homelab services follow a repeated pattern: git definition, k8s workload +
HTTPRoute, Argo CD apply, staged rollout. Today that is either hand-applied
YAML or a copy-pasted GitLab job (see ivy-universe: Kaniko build, bump a tag
in ivy-ops, `argocd app sync`, delta then gamma).

The work-inspired shape:

1. Define a **deployable** in git.
2. Get **scaffolded k8s** (Deployment/Service/HTTPRoute, Argo Application).
3. On merge to the default branch, apply via **Argo**, staged (homelab then
   production; ivy-style delta → gamma is the same idea with different names).
4. A **web UI** to see what is where and to pin / promote by hand.

This is a small-scale control plane on top of GitLab/GitHub + Argo + Gateway
API, not a replacement for them.

## Product shape

- **Git remains source of truth.** Deploybot writes git and asks Argo to sync.
  It does not `kubectl apply` app workloads.
- **Dashboard is not enough.** A UI over GitLab + Argo does not kill the
  copy-paste CI. Deploybot owns the spec → manifests → pin → health-gated
  promote loop.
- **Two app flavors, one spec:** source-built (image from CI SHA) and
  third-party (pinned upstream tag, often a StatefulSet). Play (sonarr,
  jackett, …) is first-class later; it is the simpler workload, not a
  different product. Outliers (plex PVs, nzbget HTTPRouteFilter, teamspeak
  TCP) use extra modules / overlay escape hatches.

## Stack

- Console: React Router 8 framework mode + Mantine (same as kmc / ivy-web).
- Orchestration: Go (render, git commit, Argo wait, promote). Not a React
  Router action — promotion is a minutes-long state machine.
- No Deployable CRD for v1. Git commit is the event; a controller is extra
  API surface.

## First customer

**kmc console only** (`examples/kmc.yaml`).

In scope: Deployment, Service, HTTPRoute, Argo Application docs, homelab →
prod, pin by digest/`main-<sha>` instead of floating `:main`.

Out of spec (stay as existing files / apply): `clusters.yaml`, env
ConfigMaps, `gen-secrets.sh`, impersonator SA/RBAC, kmc-controller + CRDs.

GitHub Actions keeps building; deploybot consumes the image. Do not move
Kaniko or rewrite `docker-build.yml` for this.

## What this repo already did

Phases 1–6: skeleton, spec + renderer + kmc goldens, image pin, local git
write (never push), Argo sync/health/promote, RR console.

Pin/promote **only** upsert `images:` on the stage overlay. They must not
rewrite workload YAML or shared Argo project `kustomization.yaml` files
(sandbox app list).

## Explicitly not done yet

- **`deploybot sync` (or equivalent):** adopt and keep generated manifests
  (Deployment, Service, HTTPRoute, our patches, Application YAML) in the ops
  repo, merging human bits (`configMapGenerator`, extra patches). This is
  what cutover should use — not pin.
- Cut kmc console over in `kcloud-ops` (stop floating `:main`).
- kmc controller as deployable #2.
- Play / ivy as spec customers.
- Preview apps, multi-service releases, replacing Actions/Kaniko.

## Non-goals

- Replacing Argo or the image builder.
- A generic workflow engine or Backstage.
- Multi-tenant IAM (Dex + Tailscale is enough).
- Flattening cluster vs app env: homelab/prod are clusters; ivy delta/gamma
  are app stages on homelab. The spec should allow both.
