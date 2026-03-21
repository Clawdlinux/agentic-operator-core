#!/usr/bin/env bash
# =============================================================================
#  E2E Golden Path — Single workload full lifecycle test
#
#  Creates a minimal AgentWorkload → waits for reconciliation to
#  Completed or Failed → validates status fields → cleans up.
#
#  If LLM credentials are configured, uses a real provider.
#  Otherwise, falls back to a minimal spec that tests operator reconciliation
#  without requiring external API calls (expects the workflow to start, even
#  if individual steps fail due to missing LLM).
#
#  Usage:
#    bash tests/e2e/test_golden_path.sh
#
#  Requires:
#    - Operator + Argo + shared services installed (run setup.sh first)
#
#  Exit codes:
#    0 — Test passed
#    1 — Test failed
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
HARNESS_DIR="$(cd "${SCRIPT_DIR}/../harness" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

# Source config if present
if [[ -f "${HARNESS_DIR}/config.env" ]]; then
  # shellcheck source=/dev/null
  source "${HARNESS_DIR}/config.env"
fi

# Defaults
NS_ARGO="${NS_ARGO:-argo-workflows}"
WORKLOAD_TIMEOUT="${WORKLOAD_TIMEOUT:-300}"
SLEEP_SECONDS="${SLEEP_SECONDS:-5}"
CLEANUP="${CLEANUP:-true}"

RUN_ID="$(date +%Y%m%dT%H%M%S)-golden"
WORKLOAD_NAME="e2e-golden-${RUN_ID}"
EVIDENCE_DIR="${EVIDENCE_DIR:-${REPO_ROOT}/tests/harness/evidence/${RUN_ID}}"
mkdir -p "${EVIDENCE_DIR}"

PASS=0
FAIL=0
TOTAL=0

pass() { echo "  ✅ PASS: $*"; ((PASS++)); ((TOTAL++)); }
fail_test() { echo "  ❌ FAIL: $*"; ((FAIL++)); ((TOTAL++)); }
log()  { echo "[e2e:golden $(date +%H:%M:%S)] $*"; }

# ── Determine provider config ────────────────────────────────────────────────
HAS_LLM=false
PROVIDER_BLOCK=""
MODEL_BLOCK=""

if [[ -n "${CF_ACCOUNT_ID:-}" && -n "${CF_API_TOKEN:-}" ]]; then
  HAS_LLM=true
  PROVIDER_BLOCK="
  providers:
    - name: cloudflare-workers-ai
      type: openai-compatible
      endpoint: https://api.cloudflare.com/client/v4/accounts/${CF_ACCOUNT_ID}/ai/v1
      apiKeySecret:
        name: cloudflare-workers-ai-token
        key: api-token
  modelMapping:
    analysis: \"cloudflare-workers-ai/@cf/qwen/qwen1.5-7b-chat-awq\""
  log "Using Cloudflare Workers AI for LLM calls"
elif [[ -n "${LLM_API_KEY:-}" && -n "${LLM_ENDPOINT:-}" ]]; then
  HAS_LLM=true
  LLM_MODEL="${LLM_MODEL:-gpt-3.5-turbo}"
  PROVIDER_BLOCK="
  providers:
    - name: custom-llm
      type: openai-compatible
      endpoint: ${LLM_ENDPOINT}
      apiKeySecret:
        name: llm-provider-token
        key: api-key
  modelMapping:
    analysis: \"custom-llm/${LLM_MODEL}\""
  log "Using custom LLM endpoint: ${LLM_ENDPOINT}"
else
  log "No LLM credentials — testing operator reconciliation only"
fi

# ── 1. Create AgentWorkload ──────────────────────────────────────────────────
log "Creating AgentWorkload: ${WORKLOAD_NAME}"

cat <<MANIFEST | kubectl apply -f - >/dev/null
apiVersion: agentic.clawdlinux.org/v1alpha1
kind: AgentWorkload
metadata:
  name: ${WORKLOAD_NAME}
  namespace: ${NS_ARGO}
  labels:
    e2e-run: "${RUN_ID}"
    test-type: golden-path
spec:
  mcpServerEndpoint: "https://httpbin.org/json"
  objective: "E2E golden path test: validate operator reconciliation lifecycle"
  autoApproveThreshold: "0.80"
  opaPolicy: permissive${PROVIDER_BLOCK}
MANIFEST

pass "AgentWorkload created: ${NS_ARGO}/${WORKLOAD_NAME}"

# ── 2. Verify resource exists ────────────────────────────────────────────────
log "Verifying AgentWorkload exists in API..."
sleep 2

if kubectl -n "${NS_ARGO}" get agentworkload "${WORKLOAD_NAME}" >/dev/null 2>&1; then
  pass "AgentWorkload readable from API"
else
  fail_test "AgentWorkload not found after creation"
fi

# ── 3. Wait for reconciliation ───────────────────────────────────────────────
log "Waiting for reconciliation (timeout: ${WORKLOAD_TIMEOUT}s)..."

DEADLINE=$(( $(date +%s) + WORKLOAD_TIMEOUT ))
FINAL_PHASE="<pending>"

while true; do
  PHASE="$(kubectl -n "${NS_ARGO}" get agentworkload "${WORKLOAD_NAME}" \
    -o jsonpath='{.status.phase}' 2>/dev/null || echo "<pending>")"

  if [[ "${PHASE}" == "Completed" ]]; then
    FINAL_PHASE="Completed"
    pass "Workload reached Completed phase"
    break
  fi

  if [[ "${PHASE}" == "Failed" ]]; then
    FINAL_PHASE="Failed"
    if [[ "${HAS_LLM}" == "true" ]]; then
      fail_test "Workload entered Failed phase (LLM was configured)"
    else
      # Without LLM, Failed is acceptable — it means operator DID reconcile
      pass "Workload entered Failed phase (expected without LLM credentials)"
    fi
    break
  fi

  if [[ "${PHASE}" == "Running" && "${FINAL_PHASE}" == "<pending>" ]]; then
    FINAL_PHASE="Running"
    log "  Phase: Running (operator reconciled)"
  fi

  if (( $(date +%s) >= DEADLINE )); then
    FINAL_PHASE="Timeout(${PHASE})"
    fail_test "Timed out after ${WORKLOAD_TIMEOUT}s (last phase: ${PHASE})"
    break
  fi

  sleep "${SLEEP_SECONDS}"
done

# ── 4. Validate status fields ────────────────────────────────────────────────
log "Validating status fields..."

# Check that status.phase is set
STATUS_PHASE="$(kubectl -n "${NS_ARGO}" get agentworkload "${WORKLOAD_NAME}" \
  -o jsonpath='{.status.phase}' 2>/dev/null || echo "")"
if [[ -n "${STATUS_PHASE}" ]]; then
  pass "status.phase is set: ${STATUS_PHASE}"
else
  fail_test "status.phase is empty"
fi

# Check that a Workflow CR was created (operator's primary job)
WORKFLOW_COUNT="$(kubectl -n "${NS_ARGO}" get workflows \
  -l "agentworkload=${WORKLOAD_NAME}" --no-headers 2>/dev/null | wc -l | tr -d ' ')"
if (( WORKFLOW_COUNT >= 1 )); then
  pass "Argo Workflow created (${WORKFLOW_COUNT} workflow(s))"
else
  # Also try matching by name prefix
  WORKFLOW_COUNT="$(kubectl -n "${NS_ARGO}" get workflows --no-headers 2>/dev/null | \
    grep -c "${WORKLOAD_NAME}" || echo "0")"
  if (( WORKFLOW_COUNT >= 1 )); then
    pass "Argo Workflow found by name match (${WORKFLOW_COUNT})"
  else
    fail_test "No Argo Workflow found for workload ${WORKLOAD_NAME}"
  fi
fi

# ── 5. Spec round-trip ───────────────────────────────────────────────────────
log "Validating spec round-trip..."

STORED_OBJECTIVE="$(kubectl -n "${NS_ARGO}" get agentworkload "${WORKLOAD_NAME}" \
  -o jsonpath='{.spec.objective}' 2>/dev/null || echo "")"
EXPECTED_OBJECTIVE="E2E golden path test: validate operator reconciliation lifecycle"
if [[ "${STORED_OBJECTIVE}" == "${EXPECTED_OBJECTIVE}" ]]; then
  pass "spec.objective round-trip OK"
else
  fail_test "spec.objective mismatch: got '${STORED_OBJECTIVE}'"
fi

STORED_ENDPOINT="$(kubectl -n "${NS_ARGO}" get agentworkload "${WORKLOAD_NAME}" \
  -o jsonpath='{.spec.mcpServerEndpoint}' 2>/dev/null || echo "")"
if [[ "${STORED_ENDPOINT}" == "https://httpbin.org/json" ]]; then
  pass "spec.mcpServerEndpoint round-trip OK"
else
  fail_test "spec.mcpServerEndpoint mismatch: got '${STORED_ENDPOINT}'"
fi

# ── 6. Collect evidence ──────────────────────────────────────────────────────
log "Collecting evidence to ${EVIDENCE_DIR}..."

kubectl -n "${NS_ARGO}" get agentworkload "${WORKLOAD_NAME}" -o yaml \
  > "${EVIDENCE_DIR}/agentworkload.yaml" 2>/dev/null || true
kubectl -n "${NS_ARGO}" describe agentworkload "${WORKLOAD_NAME}" \
  > "${EVIDENCE_DIR}/agentworkload.describe.txt" 2>/dev/null || true
kubectl -n "${NS_ARGO}" get workflows --no-headers \
  > "${EVIDENCE_DIR}/workflows.txt" 2>/dev/null || true
kubectl -n "${NS_ARGO}" get events --sort-by=.lastTimestamp \
  > "${EVIDENCE_DIR}/events.txt" 2>/dev/null || true
kubectl -n agentic-system logs deploy/agentic-operator --tail=200 \
  > "${EVIDENCE_DIR}/operator.logs.txt" 2>/dev/null || true

pass "Evidence collected"

# ── 7. Cleanup ────────────────────────────────────────────────────────────────
if [[ "${CLEANUP}" == "true" ]]; then
  log "Cleaning up test workload..."
  kubectl -n "${NS_ARGO}" delete agentworkload "${WORKLOAD_NAME}" \
    --ignore-not-found >/dev/null 2>&1 || true
  pass "Test workload deleted"
fi

# ── Summary ───────────────────────────────────────────────────────────────────
echo ""
echo "════════════════════════════════════════════"
echo "  Golden Path E2E: ${PASS}/${TOTAL} passed, ${FAIL} failed"
echo "  Final phase: ${FINAL_PHASE}"
echo "  Evidence: ${EVIDENCE_DIR}"
echo "════════════════════════════════════════════"

if (( FAIL > 0 )); then
  exit 1
fi

exit 0
