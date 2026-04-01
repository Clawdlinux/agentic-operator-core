#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE_FILE="$ROOT_DIR/docker-compose.yml"

require_cmd() {
  local cmd="$1"
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "ERROR: required command '$cmd' is not installed"
    exit 1
  fi
}

cleanup() {
  docker compose -f "$COMPOSE_FILE" down -v --remove-orphans >/dev/null 2>&1 || true
}

require_cmd docker
require_cmd curl
require_cmd jq
require_cmd make

if [[ -f "$ROOT_DIR/.env" ]]; then
  set -a
  # shellcheck disable=SC1091
  source "$ROOT_DIR/.env"
  set +a
fi

required_vars=(
  AZURE_OPENAI_ENDPOINT
  AZURE_OPENAI_API_KEY
  AZURE_OPENAI_API_VERSION
)

for var_name in "${required_vars[@]}"; do
  if [[ -z "${!var_name:-}" ]]; then
    echo "ERROR: required variable '$var_name' is missing"
    echo "Hint: cp .env.example .env and set Azure AI Foundry values before running validation."
    exit 1
  fi
done

if [[ "${AZURE_OPENAI_ENDPOINT}" != https://* ]]; then
  echo "ERROR: AZURE_OPENAI_ENDPOINT must start with https://"
  exit 1
fi

trap cleanup EXIT

echo "[1/6] Resetting local stack"
cleanup

echo "[2/6] Building images"
make -C "$ROOT_DIR" build

echo "[3/6] Starting services"
make -C "$ROOT_DIR" up

echo "[4/6] Waiting for health endpoints"
for _ in $(seq 1 24); do
  if curl -fsS http://localhost:9001/health >/dev/null \
    && curl -fsS http://localhost:9002/health >/dev/null \
    && curl -fsS http://localhost:9003/health >/dev/null \
    && curl -fsS http://localhost:9000/health >/dev/null \
    && curl -fsS http://localhost:8000/health >/dev/null; then
    break
  fi
  sleep 5
done

curl -fsS http://localhost:9001/health >/dev/null
curl -fsS http://localhost:9002/health >/dev/null
curl -fsS http://localhost:9003/health >/dev/null
curl -fsS http://localhost:9000/health >/dev/null
curl -fsS http://localhost:8000/health >/dev/null

echo "[5/6] Running orchestration request"
response_file="$(mktemp)"
curl -fsS -X POST http://localhost:9000/orchestrate \
  -H "Content-Type: application/json" \
  -d '{"topic":"Quantum computing breakthroughs in 2026"}' > "$response_file"

jq -e '.trace_id | length > 0' "$response_file" >/dev/null
jq -e '.stages | length == 3' "$response_file" >/dev/null
jq -e 'all(.stages[]; .status == "completed")' "$response_file" >/dev/null
jq -e '.final_output | length > 0' "$response_file" >/dev/null

total_cost="$(jq -r '.total_cost_usd // 0' "$response_file")"
if ! awk -v c="$total_cost" 'BEGIN {exit !(c > 0)}'; then
  echo "ERROR: expected total_cost_usd > 0, got: $total_cost"
  cat "$response_file"
  exit 1
fi

echo "[6/6] Checking per-agent cost endpoint"
make -C "$ROOT_DIR" cost-report

echo "Validation PASS"
echo "trace_id=$(jq -r '.trace_id' "$response_file")"
echo "total_cost_usd=$total_cost"
