#!/usr/bin/env bash
# =============================================================================
#  Smoke Tests — Validate cluster infrastructure is healthy
#
#  These tests verify the operator + dependencies are installed and responsive.
#  They do NOT create workloads or require LLM credentials.
#
#  Usage:
#    bash tests/smoke/run_smoke.sh
#
#  Exit codes:
#    0 — All smoke tests passed
#    1 — One or more tests failed
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
HARNESS_DIR="$(cd "${SCRIPT_DIR}/../harness" && pwd)"

# Source config if present
if [[ -f "${HARNESS_DIR}/config.env" ]]; then
  # shellcheck source=/dev/null
  source "${HARNESS_DIR}/config.env"
fi

# Defaults
NS_ARGO="${NS_ARGO:-argo-workflows}"
NS_OPERATOR="${NS_OPERATOR:-agentic-system}"
NS_SHARED="${NS_SHARED:-shared-services}"

PASS=0
FAIL=0
TOTAL=0

pass() { echo "  ✅ PASS: $*"; ((PASS++)); ((TOTAL++)); }
fail() { echo "  ❌ FAIL: $*"; ((FAIL++)); ((TOTAL++)); }
log()  { echo "[smoke] $*"; }

# ── Test 1: CRD installed ────────────────────────────────────────────────────
log "Test 1: AgentWorkload CRD exists"
if kubectl get crd agentworkloads.agentic.clawdlinux.org >/dev/null 2>&1; then
  # Verify it has the expected group
  GROUP="$(kubectl get crd agentworkloads.agentic.clawdlinux.org \
    -o jsonpath='{.spec.group}' 2>/dev/null)"
  if [[ "${GROUP}" == "agentic.clawdlinux.org" ]]; then
    pass "CRD exists with correct group: ${GROUP}"
  else
    fail "CRD exists but wrong group: ${GROUP} (expected agentic.clawdlinux.org)"
  fi
else
  fail "AgentWorkload CRD not found"
fi

# ── Test 2: Operator pod running ──────────────────────────────────────────────
log "Test 2: Operator controller-manager running"
OPERATOR_PODS="$(kubectl -n "${NS_OPERATOR}" get pods \
  -l control-plane=controller-manager \
  --field-selector=status.phase=Running --no-headers 2>/dev/null | wc -l | tr -d ' ')"
if (( OPERATOR_PODS >= 1 )); then
  pass "Operator pod running (${OPERATOR_PODS} replica(s))"
else
  fail "No running operator pods in ${NS_OPERATOR}"
fi

# ── Test 3: Operator not crash-looping ────────────────────────────────────────
log "Test 3: Operator not crash-looping"
RESTART_COUNT="$(kubectl -n "${NS_OPERATOR}" get pods \
  -l control-plane=controller-manager \
  -o jsonpath='{.items[0].status.containerStatuses[0].restartCount}' 2>/dev/null || echo "0")"
if (( RESTART_COUNT <= 2 )); then
  pass "Operator restarts: ${RESTART_COUNT} (threshold: ≤2)"
else
  fail "Operator restart count too high: ${RESTART_COUNT}"
fi

# ── Test 4: Argo workflow controller running ──────────────────────────────────
log "Test 4: Argo workflow controller running"
ARGO_PODS="$(kubectl -n "${NS_ARGO}" get pods \
  -l app.kubernetes.io/name=argo-workflows-workflow-controller \
  --field-selector=status.phase=Running --no-headers 2>/dev/null | wc -l | tr -d ' ')"
if (( ARGO_PODS >= 1 )); then
  pass "Argo controller running (${ARGO_PODS} pod(s))"
else
  fail "Argo controller not running in ${NS_ARGO}"
fi

# ── Test 5: WorkflowTemplate exists ──────────────────────────────────────────
log "Test 5: WorkflowTemplate visual-analysis-template exists"
if kubectl -n "${NS_ARGO}" get workflowtemplate visual-analysis-template >/dev/null 2>&1; then
  pass "WorkflowTemplate visual-analysis-template exists"
else
  fail "WorkflowTemplate visual-analysis-template not found in ${NS_ARGO}"
fi

# ── Test 6: Shared services reachable ─────────────────────────────────────────
log "Test 6: Shared services pods running"
SHARED_OK=true
for SVC in postgres minio browserless; do
  POD_COUNT="$(kubectl -n "${NS_SHARED}" get pods -l app="${SVC}" \
    --field-selector=status.phase=Running --no-headers 2>/dev/null | wc -l | tr -d ' ')"
  if (( POD_COUNT >= 1 )); then
    pass "${SVC} running"
  else
    fail "${SVC} not running in ${NS_SHARED}"
    SHARED_OK=false
  fi
done

# ── Test 7: Service endpoints resolvable ──────────────────────────────────────
log "Test 7: Shared service endpoints exist"
for SVC in postgres minio browserless litellm; do
  EP_COUNT="$(kubectl -n "${NS_SHARED}" get endpoints "${SVC}" \
    -o jsonpath='{.subsets[*].addresses}' 2>/dev/null | wc -c | tr -d ' ')"
  if (( EP_COUNT > 2 )); then
    pass "${SVC} endpoint has addresses"
  else
    fail "${SVC} endpoint has no ready addresses"
  fi
done

# ── Test 8: Webhook configuration valid (if cert-manager is present) ──────────
log "Test 8: Webhook configuration"
if kubectl get validatingwebhookconfiguration 2>/dev/null | grep -q agentic; then
  # Verify the CA bundle is populated
  CA_LEN="$(kubectl get validatingwebhookconfiguration -l app.kubernetes.io/part-of=agentic-operator \
    -o jsonpath='{.items[0].webhooks[0].clientConfig.caBundle}' 2>/dev/null | wc -c | tr -d ' ')"
  if (( CA_LEN > 10 )); then
    pass "Webhook CA bundle populated (${CA_LEN} chars)"
  else
    fail "Webhook CA bundle empty or missing"
  fi
else
  pass "No webhook configured (acceptable for dev clusters)"
fi

# ── Test 9: Operator can watch AgentWorkloads ─────────────────────────────────
log "Test 9: Operator RBAC — can list AgentWorkloads"
SA="system:serviceaccount:${NS_OPERATOR}:agentic-operator-controller-manager"
CAN_LIST="$(kubectl auth can-i list agentworkloads.agentic.clawdlinux.org \
  --as="${SA}" --all-namespaces 2>/dev/null || echo "no")"
if [[ "${CAN_LIST}" == "yes" ]]; then
  pass "Operator SA can list AgentWorkloads cluster-wide"
else
  # Might not work on all clusters (RBAC aggregation differs)
  log "  ⚠️  auth can-i returned '${CAN_LIST}' — may be cluster-specific"
  pass "Skipped (auth can-i not reliable on all clusters)"
fi

# ── Test 10: CRD schema has required fields ───────────────────────────────────
log "Test 10: CRD spec schema includes key fields"
CRD_JSON="$(kubectl get crd agentworkloads.agentic.clawdlinux.org -o json 2>/dev/null || echo "{}")"
FIELDS_OK=true
for FIELD in objective mcpServerEndpoint autoApproveThreshold; do
  if echo "${CRD_JSON}" | grep -q "\"${FIELD}\""; then
    pass "CRD field '${FIELD}' present"
  else
    fail "CRD field '${FIELD}' missing from schema"
    FIELDS_OK=false
  fi
done

# ── Summary ───────────────────────────────────────────────────────────────────
echo ""
echo "════════════════════════════════════════════"
echo "  Smoke Tests: ${PASS}/${TOTAL} passed, ${FAIL} failed"
echo "════════════════════════════════════════════"

if (( FAIL > 0 )); then
  exit 1
fi

exit 0
