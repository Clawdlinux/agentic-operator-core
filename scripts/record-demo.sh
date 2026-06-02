#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "${ROOT_DIR}"

mkdir -p demos
STAMP="$(date +%Y%m%d)"
LOG_FILE="demos/demo-${STAMP}.log"
CAST_FILE="demos/demo-${STAMP}.cast"
DEMO_CMD="set -o pipefail; ./scripts/demo-booth.sh --skip-argo | tee ${LOG_FILE}"

if command -v asciinema >/dev/null 2>&1; then
  asciinema rec "${CAST_FILE}" --command "bash -lc '${DEMO_CMD}'"
else
  bash -lc "${DEMO_CMD}"
fi

echo "Demo recording saved to demos/"
