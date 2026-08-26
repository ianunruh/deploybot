set dotenv-load := false

go_files := "./..."

build:
    mkdir -p build
    go build -o build/deploybot .

test:
    go test {{go_files}}

golangci := `echo "$(go env GOPATH)/bin/golangci-lint"`

lint:
    {{golangci}} run
    cd web && pnpm lint
    cd web && pnpm format:check

fmt:
    {{golangci}} fmt
    cd web && pnpm format

typecheck:
    cd web && pnpm typecheck

check: lint test typecheck

web-install:
    cd web && pnpm install

web-dev:
    cd web && pnpm dev

docker:
    docker build -t ghcr.io/ianunruh/deploybot:local -f Dockerfile .
    docker build -t ghcr.io/ianunruh/deploybot-web:local -f web/Dockerfile web

serve *args:
    go run . serve {{args}}

valkey:
    docker compose up -d --wait valkey

# API on :8080 and the React Router console on :5173. Starts local Valkey.
dev:
    #!/usr/bin/env bash
    set -euo pipefail
    docker compose up -d --wait valkey
    go run . serve --addr 127.0.0.1:8080 &
    api=$!
    trap 'kill "$api"' EXIT
    cd web && pnpm dev
