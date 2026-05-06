#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

PROJECT_NAME="${COMPOSE_PROJECT_NAME:-rinha-ci}"
PUBLIC_PORT_VALUE="${PUBLIC_PORT:-9999}"
ARTIFACTS_DIR="${ARTIFACTS_DIR:-.artifacts/official-tests}"
OFFICIAL_REPO_URL="${OFFICIAL_REPO_URL:-https://github.com/zanfranceschi/rinha-de-backend-2026.git}"
OFFICIAL_REF="${OFFICIAL_REF:-main}"
SKIP_COMPOSE_BUILD=0
SKIP_VERIFY=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --skip-compose-build) SKIP_COMPOSE_BUILD=1 ;;
    --skip-verify) SKIP_VERIFY=1 ;;
    --project-name) PROJECT_NAME="$2"; shift ;;
    --public-port) PUBLIC_PORT_VALUE="$2"; shift ;;
    *) echo "unknown argument: $1" >&2; exit 2 ;;
  esac
  shift
done

export COMPOSE_PROJECT_NAME="$PROJECT_NAME"
export PUBLIC_PORT="$PUBLIC_PORT_VALUE"

NETWORK_NAME="${PROJECT_NAME}-app-network"
OFFICIAL_SRC_DIR="$ARTIFACTS_DIR/official-src"
K6_WORK_DIR="$ARTIFACTS_DIR/k6"
RESULTS_FILE="$K6_WORK_DIR/test/results.json"
mkdir -p "$ARTIFACTS_DIR"

cleanup() {
  docker compose ps > "$ARTIFACTS_DIR/compose-ps.txt" 2>&1 || true
  docker compose logs --no-color > "$ARTIFACTS_DIR/compose.log" 2>&1 || true
  docker stats --no-stream > "$ARTIFACTS_DIR/docker-stats.txt" 2>&1 || true
  docker compose config > "$ARTIFACTS_DIR/docker-compose-config.yml" 2>&1 || true
  if [[ -f "$RESULTS_FILE" ]]; then
    cp "$RESULTS_FILE" "$ARTIFACTS_DIR/results.json"
  fi
  docker compose down -v --remove-orphans > "$ARTIFACTS_DIR/compose-down.txt" 2>&1 || true
}
trap cleanup EXIT

if [[ "$SKIP_VERIFY" -eq 0 ]]; then
  bash ./scripts/verify.sh
fi

rm -rf "$OFFICIAL_SRC_DIR" "$K6_WORK_DIR"
git clone --depth 1 --branch "$OFFICIAL_REF" "$OFFICIAL_REPO_URL" "$OFFICIAL_SRC_DIR"

mkdir -p "$K6_WORK_DIR/test"
cp "$OFFICIAL_SRC_DIR/test/smoke.js" "$K6_WORK_DIR/test/smoke.js"
cp "$OFFICIAL_SRC_DIR/test/test.js" "$K6_WORK_DIR/test/test.js"
cp "$OFFICIAL_SRC_DIR/test/test-data.json" "$K6_WORK_DIR/test/test-data.json"
sed -i.bak 's|http://localhost:9999|http://nginx|g' "$K6_WORK_DIR/test/smoke.js" "$K6_WORK_DIR/test/test.js"
rm -f "$K6_WORK_DIR/test/"*.bak
: > "$RESULTS_FILE"
chmod -R a+rwX "$K6_WORK_DIR" || true

DOCKER_USER_ARGS=()
if command -v id >/dev/null 2>&1; then
  DOCKER_USER_ARGS=(--user "$(id -u):$(id -g)")
fi

if [[ "$SKIP_COMPOSE_BUILD" -eq 1 ]]; then
  docker compose up -d --no-build
else
  docker compose up -d --build
fi

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

assert_shard() {
  local service="$1"
  local shard="$2"
  local expected_vec_bytes="$3"
  local expected_label_bytes="$4"
  local vec_path="/app/resources/${shard}/references.vec"
  local label_path="/app/resources/${shard}/references.labels"

  docker compose exec -T "$service" sh -lc "test -f '$vec_path' && test -f '$label_path'"

  local vec_bytes
  vec_bytes="$(docker compose exec -T "$service" sh -lc "wc -c < '$vec_path'" | tr -d '\r[:space:]')"
  local label_bytes
  label_bytes="$(docker compose exec -T "$service" sh -lc "wc -c < '$label_path'" | tr -d '\r[:space:]')"

  if [[ "$vec_bytes" != "$expected_vec_bytes" ]]; then
    echo "unexpected vec bytes for $service shard $shard: got $vec_bytes want $expected_vec_bytes" >&2
    return 1
  fi

  if [[ "$label_bytes" != "$expected_label_bytes" ]]; then
    echo "unexpected label bytes for $service shard $shard: got $label_bytes want $expected_label_bytes" >&2
    return 1
  fi
}

wait_for_health app1
wait_for_health app2
wait_for_health nginx
wait_for_ready "http://127.0.0.1:${PUBLIC_PORT_VALUE}/ready"

assert_shard app1 0 84000000 1500000
assert_shard app2 1 84000000 1500000

docker run --rm \
  "${DOCKER_USER_ARGS[@]}" \
  --network "$NETWORK_NAME" \
  -e K6_NO_USAGE_REPORT=true \
  -v "$ROOT_DIR/$K6_WORK_DIR:/work" \
  -w /work \
  grafana/k6:latest run /work/test/smoke.js | tee "$ARTIFACTS_DIR/k6-smoke.txt"

docker run --rm \
  "${DOCKER_USER_ARGS[@]}" \
  --network "$NETWORK_NAME" \
  -e K6_NO_USAGE_REPORT=true \
  -v "$ROOT_DIR/$K6_WORK_DIR:/work" \
  -w /work \
  grafana/k6:latest run /work/test/test.js | tee "$ARTIFACTS_DIR/k6-official.txt"

go run ./cmd/officialcheck -file "$RESULTS_FILE" | tee "$ARTIFACTS_DIR/official-summary.txt"
