# deploybot

Small-scale release control plane: a deployable spec in git becomes
Deployment / Service / HTTPRoute / Argo Application, then you pin an image
digest and promote homelab → prod.

The first customer is the kmc console. ConfigMaps, secrets, impersonator RBAC,
and the kmc controller are not generated.

Original intent and non-goals: [docs/goals.md](docs/goals.md).

## Layout

| Path | Role |
|------|------|
| `internal/` | Go: spec, render, pin, git write, Argo, HTTP API |
| `web/` | React Router 8 + Mantine console |
| `examples/` | Deployable specs (start with `kmc.yaml`) |

## Develop

```bash
just web-install
just check
just dev          # API :8080 + console :5173
```

CLI (local git commits only; nothing is pushed):

```bash
just build
./build/deploybot render examples/kmc.yaml --out /tmp/kmc-out
./build/deploybot pin --spec examples/kmc.yaml --stage homelab \
  --image ghcr.io/ianunruh/kmc@sha256:… --repo /path/to/kcloud-ops --apply
./build/deploybot promote --spec examples/kmc.yaml --from homelab --to prod \
  --repo /path/to/kcloud-ops --apply
./build/deploybot sync --spec examples/kmc.yaml --stage homelab \
  --repo /path/to/kcloud-ops --apply
```

`--apply` commits locally. `--sync` talks to Argo (`DEPLOYBOT_ARGO_URL` /
`DEPLOYBOT_ARGO_URL_<STAGE>`). Default is dry-run.

The pin picker lists GHCR versions (newest first) via the GitHub Packages
API. Auth is `DEPLOYBOT_GITHUB_TOKEN`, `GH_TOKEN`, `GITHUB_TOKEN`, or
`gh auth token`. The token needs `read:packages` (or `write:packages`).
Without that scope it falls back to `main-<sha>` tags from git history.

Lint Go with golangci-lint v2.13+ built with Go 1.27 (Homebrew 2.10 is too old
for this toolchain):

```bash
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.1
```
