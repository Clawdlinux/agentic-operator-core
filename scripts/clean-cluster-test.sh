#!/usr/bin/env bash
set -euo pipefail

GREEN="$(printf '\033[32m')"
RED="$(printf '\033[31m')"
RESET="$(printf '\033[0m')"
CLUSTER="ninevigil-demo"
cleanup() { kind delete cluster --name "${CLUSTER}" >/dev/null 2>&1 || true; }
trap cleanup EXIT

kind delete cluster --name "${CLUSTER}" || true
kind create cluster --name "${CLUSTER}" --wait 60s

set +e
./scripts/demo-booth.sh --skip-argo
exit_code=$?
set -e
if (( exit_code == 0 )); then
  printf '%bCLEAN CLUSTER TEST: PASS%b\n' "${GREEN}" "${RESET}"
else
  printf '%bCLEAN CLUSTER TEST: FAIL%b\n' "${RED}" "${RESET}"
fi

exit "${exit_code}"
