#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "${ROOT_DIR}"

mkdir -p demos
STAMP="$(date +%Y%m%d)"
LOG_FILE="demos/demo-${STAMP}.log"
CAST_FILE="demos/demo-${STAMP}.cast"
DEMO_ARGS=(--present)
if [[ "${DEMO_TAMPER_AUDIT:-false}" == "true" ]]; then
  DEMO_ARGS+=(--tamper-audit)
fi

printf -v DEMO_RUNNER '%q ' env FORCE_COLOR=1 ./scripts/demo-booth.sh "${DEMO_ARGS[@]}"
printf -v QUOTED_LOG_FILE '%q' "${LOG_FILE}"
DEMO_CMD="set -o pipefail; ${DEMO_RUNNER}| tee ${QUOTED_LOG_FILE}"

if command -v asciinema >/dev/null 2>&1; then
  printf -v ASCIINEMA_COMMAND 'bash -lc %q' "${DEMO_CMD}"
  asciinema rec "${CAST_FILE}" --command "${ASCIINEMA_COMMAND}"
else
  bash -lc "${DEMO_CMD}"
fi

echo "Demo recording saved to demos/"
