#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

ARTIFACTS_DIR="${ARTIFACTS_DIR:-.artifacts/bench}"
BASELINE_FILE="${BASELINE_FILE:-}"
mkdir -p "$ARTIFACTS_DIR"

CURRENT_FILE="$ARTIFACTS_DIR/current.txt"
BENCHSTAT_FILE="$ARTIFACTS_DIR/benchstat.txt"

echo "==> go test -run '^$' -bench . -benchmem ./internal/score ./internal/vector"
go test -run '^$' -bench . -benchmem ./internal/score ./internal/vector | tee "$CURRENT_FILE"

if [[ -n "$BASELINE_FILE" && -f "$BASELINE_FILE" ]]; then
  echo "==> go run golang.org/x/perf/cmd/benchstat@latest \"$BASELINE_FILE\" \"$CURRENT_FILE\""
  go run golang.org/x/perf/cmd/benchstat@latest "$BASELINE_FILE" "$CURRENT_FILE" | tee "$BENCHSTAT_FILE"
fi
