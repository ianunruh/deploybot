# deploybot

Small-scale release control plane: a deployable spec in git becomes a
Deployment and Argo Application (plus Service / HTTPRoute when the app is
routed), then you pin an image digest and promote homelab → prod.

Customers today: kmc console (`examples/kmc.yaml`) and kmc-controller
(`examples/kmc-controller.yaml`). Deploybot generates the skeleton and the
image pin. ConfigMaps, secrets, CRDs, RBAC, and extra pod fields (env,
volumes, args, securityContext, CIDRs) stay as extra files / overlay patches.

Original intent and non-goals: [docs/goals.md](docs/goals.md).

## Layout

| Path | Role |
|------|------|
| `internal/` | Go: spec, render, pin, git write, Argo, HTTP API |
| `web/` | React Router 8 + Mantine console |
| `examples/` | Deployable specs and overlay patches |
| `deploybot.yaml` | Process config (Argo URLs). Secrets stay in env / token files |

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
./build/deploybot sync --spec examples/kmc.yaml --stage homelab \
  --repo /path/to/kcloud-ops --apply --push
```

`--apply` commits locally. `--push` (requires `--apply`) pushes the current
branch; it never force-pushes. `--sync` talks to Argo. Default is dry-run. For
`serve`, set `DEPLOYBOT_APPLY=1`, `DEPLOYBOT_PUSH=1`, and `DEPLOYBOT_SYNC=1`.

Process config is YAML (`deploybot.yaml` or `--config` / `DEPLOYBOT_CONFIG`).
Flags override env, env overrides the file. Argo CD origins live in the file:

```yaml
argo:
  homelab:
    url: https://argocd.k8s.kcloud.zone
  prod:
    url: https://argocd.k8s.kcloud.io
    # tokenFile: secrets/argocd-prod.token
    # tokenEnv: DEPLOYBOT_ARGO_TOKEN_PROD
```

Tokens stay out of YAML values: `tokenFile`, `tokenEnv`, or
`DEPLOYBOT_ARGO_TOKEN` / `DEPLOYBOT_ARGO_TOKEN_<STAGE>`.
`DEPLOYBOT_ARGO_URL_<STAGE>` still overrides a stage URL.

The pin picker lists GHCR versions (newest first) via the GitHub Packages
API. Auth is `DEPLOYBOT_GITHUB_TOKEN`, `GH_TOKEN`, `GITHUB_TOKEN`, or
`gh auth token`. The token needs `read:packages` (or `write:packages`).
Without that scope it falls back to `main-<sha>` tags from git history.

HTTPS git push uses `DEPLOYBOT_GIT_TOKEN`, then the same GitHub tokens as
the pin picker. SSH remotes use the ssh-agent (or `~/.ssh/id_ed25519` /
`id_rsa`).

Lint Go with golangci-lint v2.13+ built with Go 1.27 (Homebrew 2.10 is too old
for this toolchain):

```bash
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.1
```
