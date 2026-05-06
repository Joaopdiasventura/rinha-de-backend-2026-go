#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

SKIP_RACE=0
if [[ "${1:-}" == "--skip-race" ]]; then
  SKIP_RACE=1
fi

echo "==> go build ./cmd/main.go"
go build ./cmd/main.go

echo "==> go test ./..."
go test ./...

if [[ "$SKIP_RACE" -eq 0 ]]; then
  if [[ "$(uname -s)" == "Linux" ]]; then
    echo "==> CGO_ENABLED=1 go test -race ./..."
    CGO_ENABLED=1 go test -race ./...
  else
    echo "==> skipping go test -race ./... outside Linux; CI workflow enforces it"
  fi
fi
