#!/usr/bin/env bash
# =============================================================================
#  run-all.sh — Full test cycle orchestrator
#
#  Single entry point: provide KUBECONFIG, run everything.
#
#  Layers:
#    1. Preflight — cluster reachability + CLI tools
#    2. Setup    — install operator + Argo + shared services
#    3. Smoke    — infrastructure health checks
#    4. E2E      — golden path + multi-tenant isolation
#    5. Evidence — collect and summarize
#    6. Teardown — clean removal (optional)
#
#  Usage:
#    export KUBECONFIG=/path/to/kubeconfig
#    bash tests/harness/run-all.sh
#
#  Environment:
#    SKIP_SETUP=true     Skip setup (use existing installation)
#    SKIP_TEARDOWN=true  Skip teardown (keep resources for debugging)
#    TEST_SUITE=smoke    Run only smoke tests (smoke|e2e|all, default: all)
#
#  Exit codes:
#    0 — All tests passed
#    1 — One or more stages failed
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

# Source config if present
if [[ -f "${SCRIPT_DIR}/config.env" ]]; then
  # shellcheck source=/dev/null
  source "${SCRIPT_DIR}/config.env"
fi

SKIP_SETUP="${SKIP_SETUP:-false}"
SKIP_TEARDOWN="${SKIP_TEARDOWN:-false}"
TEST_SUITE="${TEST_SUITE:-all}"

RUN_ID="$(date +%Y%m%dT%H%M%S)-full"
export EVIDENCE_DIR="${EVIDENCE_DIR:-${REPO_ROOT}/tests/harness/evidence/${RUN_ID}}"
mkdir -p "${EVIDENCE_DIR}"

STAGE_RESULTS=()
OVERALL_EXIT=0

log() { echo ""; echo "═══════════════════════════════════════════════════════════"; echo "  $*"; echo "═══════════════════════════════════════════════════════════"; }

run_stage() {
  local name="$1" script="$2"
  log "Stage: ${name}"
  if bash "${script}"; then
    STAGE_RESULTS+=("✅ ${name}")
  else
    STAGE_RESULTS+=("❌ ${name}")
    OVERALL_EXIT=1
  fi
}

# ── Banner ────────────────────────────────────────────────────────────────────
echo ""
echo "╔══════════════════════════════════════════════════════════════════════╗"
echo "║  Agentic Operator — Full Test Cycle                                 ║"
echo "╠══════════════════════════════════════════════════════════════════════╣"
echo "║  Run ID:     ${RUN_ID}"
echo "║  Suite:      ${TEST_SUITE}"
echo "║  Cluster:    $(kubectl config current-context 2>/dev/null || echo '<no context>')"
echo "║  Evidence:   ${EVIDENCE_DIR}"
echo "║  Skip setup: ${SKIP_SETUP}"
echo "╚══════════════════════════════════════════════════════════════════════╝"

START_TIME="$(date +%s)"

# ── 1. Setup ──────────────────────────────────────────────────────────────────
if [[ "${SKIP_SETUP}" != "true" ]]; then
  run_stage "Setup" "${SCRIPT_DIR}/setup.sh"
  if (( OVERALL_EXIT != 0 )); then
    echo "Setup failed — aborting test run"
    exit 1
  fi
else
  log "Stage: Setup (SKIPPED)"
  STAGE_RESULTS+=("⏭️  Setup (skipped)")
  # Still run preflight to validate existing installation
  run_stage "Preflight" "${SCRIPT_DIR}/preflight.sh"
fi

# ── 2. Smoke Tests ───────────────────────────────────────────────────────────
if [[ "${TEST_SUITE}" == "all" || "${TEST_SUITE}" == "smoke" ]]; then
  run_stage "Smoke Tests" "${REPO_ROOT}/tests/smoke/run_smoke.sh"
fi

# ── 3. E2E Tests ─────────────────────────────────────────────────────────────
if [[ "${TEST_SUITE}" == "all" || "${TEST_SUITE}" == "e2e" ]]; then
  run_stage "E2E: Golden Path" "${REPO_ROOT}/tests/e2e/test_golden_path.sh"
  run_stage "E2E: Multi-Tenant" "${REPO_ROOT}/tests/e2e/test_multi_tenant.sh"
fi

# ── 4. Evidence summary ──────────────────────────────────────────────────────
log "Stage: Evidence Summary"

# Capture cluster state summary
{
  echo "# Test Run: ${RUN_ID}"
  echo "# Cluster: $(kubectl config current-context 2>/dev/null || echo '<unknown>')"
  echo "# Date: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
  echo ""
  echo "## Nodes"
  kubectl get nodes -o wide 2>/dev/null || echo "(unavailable)"
  echo ""
  echo "## Namespaces"
  kubectl get namespaces 2>/dev/null || echo "(unavailable)"
  echo ""
  echo "## AgentWorkloads (all namespaces)"
  kubectl get agentworkloads --all-namespaces 2>/dev/null || echo "(none)"
  echo ""
  echo "## Operator Pod"
  kubectl -n agentic-system get pods -l control-plane=controller-manager -o wide 2>/dev/null || echo "(unavailable)"
  echo ""
  echo "## Argo Workflows"
  kubectl -n argo-workflows get workflows --no-headers 2>/dev/null || echo "(none)"
} > "${EVIDENCE_DIR}/cluster_summary.txt" 2>/dev/null

STAGE_RESULTS+=("✅ Evidence Summary")

# ── 5. Teardown ──────────────────────────────────────────────────────────────
if [[ "${SKIP_TEARDOWN}" != "true" && "${SKIP_SETUP}" != "true" ]]; then
  TEARDOWN_LEVEL="${TEARDOWN_LEVEL:-tests}" run_stage "Teardown" "${SCRIPT_DIR}/teardown.sh"
else
  log "Stage: Teardown (SKIPPED)"
  STAGE_RESULTS+=("⏭️  Teardown (skipped)")
fi

# ── Final Report ──────────────────────────────────────────────────────────────
END_TIME="$(date +%s)"
DURATION=$(( END_TIME - START_TIME ))

echo ""
echo ""
echo "╔══════════════════════════════════════════════════════════════════════╗"
echo "║  Full Test Cycle — Final Report                                      ║"
echo "╠══════════════════════════════════════════════════════════════════════╣"
for RESULT in "${STAGE_RESULTS[@]}"; do
  printf "║  %-66s ║\n" "${RESULT}"
done
echo "╠══════════════════════════════════════════════════════════════════════╣"
printf "║  Duration: %ds                                                      ║\n" "${DURATION}"
printf "║  Evidence: %-56s ║\n" "${EVIDENCE_DIR}"
echo "╚══════════════════════════════════════════════════════════════════════╝"

# Write machine-readable results
{
  echo "run_id=${RUN_ID}"
  echo "exit_code=${OVERALL_EXIT}"
  echo "duration_seconds=${DURATION}"
  echo "suite=${TEST_SUITE}"
  for i in "${!STAGE_RESULTS[@]}"; do
    echo "stage_${i}=${STAGE_RESULTS[$i]}"
  done
} > "${EVIDENCE_DIR}/results.env"

exit "${OVERALL_EXIT}"
