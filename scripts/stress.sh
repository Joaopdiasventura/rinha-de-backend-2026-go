#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

PROJECT_NAME="${COMPOSE_PROJECT_NAME:-rinha-stress}"
PUBLIC_PORT_VALUE="${PUBLIC_PORT:-9999}"
ARTIFACTS_DIR="${ARTIFACTS_DIR:-.artifacts/stress}"
BUILD_STACK=0
SKIP_VERIFY=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --build) BUILD_STACK=1 ;;
    --skip-verify) SKIP_VERIFY=1 ;;
    --project-name) PROJECT_NAME="$2"; shift ;;
    --public-port) PUBLIC_PORT_VALUE="$2"; shift ;;
    *) echo "unknown argument: $1" >&2; exit 2 ;;
  esac
  shift
done

export COMPOSE_PROJECT_NAME="$PROJECT_NAME"
export PUBLIC_PORT="$PUBLIC_PORT_VALUE"
export DIAGNOSTICS_ENABLED=1
export DIAGNOSTICS_PORT=6060

NETWORK_NAME="${PROJECT_NAME}-app-network"
ARTIFACTS_DIR_ABS="$ROOT_DIR/$ARTIFACTS_DIR"
STRESS_FAILED=0
mkdir -p "$ARTIFACTS_DIR_ABS"

cleanup() {
  docker compose ps > "$ARTIFACTS_DIR_ABS/compose-ps.txt" 2>&1 || true
  docker compose logs --no-color > "$ARTIFACTS_DIR_ABS/compose.log" 2>&1 || true
  docker stats --no-stream > "$ARTIFACTS_DIR_ABS/docker-stats.txt" 2>&1 || true
  docker compose down -v --remove-orphans > "$ARTIFACTS_DIR_ABS/compose-down.txt" 2>&1 || true
}
trap cleanup EXIT

wait_for_health() {
  local service="$1"
  for _ in $(seq 1 60); do
    local container_id
    container_id="$(docker compose ps -q "$service")"
    if [[ -n "$container_id" ]]; then
      local status
      status="$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "$container_id")"
      if [[ "$status" == "healthy" || "$status" == "running" ]]; then
        return 0
      fi
    fi
    sleep 2
  done
  echo "service $service did not become healthy" >&2
  return 1
}

wait_for_ready() {
  local url="$1"
  for _ in $(seq 1 60); do
    if curl --fail --silent --show-error "$url" > /dev/null; then
      return 0
    fi
    sleep 2
  done
  echo "endpoint did not become ready: $url" >&2
  return 1
}

capture_runtime() {
  local service="$1"
  local output_file="$2"
  docker compose exec -T "$service" wget -qO- "http://127.0.0.1:6060/debug/runtime/metrics" > "$output_file"
}

capture_profile() {
  local service="$1"
  local path="$2"
  local output_file="$3"
  docker compose exec -T "$service" sh -lc "wget -qO- '$path'" > "$output_file"
}

run_k6() {
  local scenario="$1"
  local rate="$2"
  local duration="$3"
  local summary_file="$ARTIFACTS_DIR_ABS/${scenario}-summary.json"
  docker run --rm \
    --network "$NETWORK_NAME" \
    -e K6_NO_USAGE_REPORT=true \
    -e TARGET_BASE_URL=http://nginx \
    -e RINHA_SCENARIO="$scenario" \
    -e RINHA_RATE="$rate" \
    -e RINHA_DURATION="$duration" \
    -e RINHA_PRE_ALLOCATED_VUS=100 \
    -e RINHA_MAX_VUS=250 \
    -e RETRY_ON_503=1 \
    -e RETRY_DELAY_MS=50 \
    -v "$ROOT_DIR/test/load:/load" \
    -v "$ARTIFACTS_DIR_ABS:/artifacts" \
    grafana/k6:latest run --summary-export "/artifacts/${scenario}-summary.json" /load/stress.js | tee "$ARTIFACTS_DIR_ABS/${scenario}.txt"
  local k6_exit="${PIPESTATUS[0]}"
  if [[ "$k6_exit" -ne 0 ]]; then
    STRESS_FAILED=1
  fi

  if ! go run ./cmd/loadreport -file "$summary_file" | tee "$ARTIFACTS_DIR_ABS/${scenario}-report.txt"; then
    STRESS_FAILED=1
  fi
}

if [[ "$SKIP_VERIFY" -eq 0 ]]; then
  bash ./scripts/verify.sh
fi

bash ./scripts/bench.sh

if [[ "$BUILD_STACK" -eq 1 ]]; then
  docker compose up -d --build
else
  docker compose up -d
fi

wait_for_health app1
wait_for_health app2
wait_for_health nginx
wait_for_ready "http://127.0.0.1:${PUBLIC_PORT_VALUE}/ready"

capture_runtime app1 "$ARTIFACTS_DIR_ABS/app1-runtime-before.json"
capture_runtime app2 "$ARTIFACTS_DIR_ABS/app2-runtime-before.json"

run_k6 burst 300 20s
run_k6 ramp 600 30s
run_k6 soak 150 45s

(
  sleep 5
  docker compose stop app2
  sleep 5
  docker compose start app2
) &
CHAOS_PID=$!
run_k6 degrade 120 20s
wait "$CHAOS_PID"
wait_for_health app2

capture_profile app1 "http://127.0.0.1:6060/debug/pprof/heap?gc=1" "$ARTIFACTS_DIR_ABS/app1-heap.pb.gz"
capture_profile app1 "http://127.0.0.1:6060/debug/pprof/profile?seconds=10" "$ARTIFACTS_DIR_ABS/app1-cpu.pb.gz"
capture_profile app1 "http://127.0.0.1:6060/debug/pprof/trace?seconds=5" "$ARTIFACTS_DIR_ABS/app1-trace.out"

capture_runtime app1 "$ARTIFACTS_DIR_ABS/app1-runtime-after.json"
capture_runtime app2 "$ARTIFACTS_DIR_ABS/app2-runtime-after.json"

if ! go run ./cmd/runtimecompare -before "$ARTIFACTS_DIR_ABS/app1-runtime-before.json" -after "$ARTIFACTS_DIR_ABS/app1-runtime-after.json" | tee "$ARTIFACTS_DIR_ABS/app1-runtime-check.txt"; then
  STRESS_FAILED=1
fi

if ! go run ./cmd/runtimecompare -before "$ARTIFACTS_DIR_ABS/app2-runtime-before.json" -after "$ARTIFACTS_DIR_ABS/app2-runtime-after.json" | tee "$ARTIFACTS_DIR_ABS/app2-runtime-check.txt"; then
  STRESS_FAILED=1
fi

exit "$STRESS_FAILED"
