#!/usr/bin/env bash
# =============================================================================
#  E2E Multi-Tenant Isolation — 3 workloads, 3 namespaces, RBAC denial
#
#  Validates:
#    1. Operator reconciles workloads across multiple namespaces
#    2. RBAC cross-namespace denial (tenant-A cannot read tenant-B)
#    3. Spec fields survive the full round-trip through etcd
#    4. Concurrent workloads don't interfere with each other
#
#  If LLM credentials are available, tests with real model calls.
#  Otherwise tests operator reconciliation + RBAC isolation only.
#
#  Usage:
#    bash tests/e2e/test_multi_tenant.sh
#
#  Requires:
#    - Operator + Argo + shared services installed (run setup.sh first)
#
#  Exit codes:
#    0 — All tests passed
#    1 — One or more tests failed
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

RUN_ID="$(date +%Y%m%dT%H%M%S)-multi"
TENANT_A_NS="agentic-customer-tenant-a"
TENANT_B_NS="agentic-customer-tenant-b"

EVIDENCE_DIR="${EVIDENCE_DIR:-${REPO_ROOT}/tests/harness/evidence/${RUN_ID}}"
mkdir -p "${EVIDENCE_DIR}"

PASS=0
FAIL=0
TOTAL=0

pass() { echo "  ✅ PASS: $*"; ((PASS++)); ((TOTAL++)); }
fail_test() { echo "  ❌ FAIL: $*"; ((FAIL++)); ((TOTAL++)); }
log()  { echo "[e2e:multi $(date +%H:%M:%S)] $*"; }

# ── 0. Setup tenant namespaces ───────────────────────────────────────────────
log "Creating tenant namespaces..."
for NS in "${TENANT_A_NS}" "${TENANT_B_NS}"; do
  kubectl create namespace "${NS}" --dry-run=client -o yaml | kubectl apply -f - >/dev/null
done
pass "Tenant namespaces created"

# Propagate LLM secrets to tenant namespaces if available
if [[ -n "${CF_API_TOKEN:-}" ]]; then
  for NS in "${TENANT_A_NS}" "${TENANT_B_NS}"; do
    kubectl create secret generic cloudflare-workers-ai-token \
      -n "${NS}" \
      --from-literal=api-token="${CF_API_TOKEN}" \
      --dry-run=client -o yaml | kubectl apply -f - >/dev/null
  done
  log "  LLM secrets propagated to tenant namespaces"
elif [[ -n "${LLM_API_KEY:-}" ]]; then
  for NS in "${TENANT_A_NS}" "${TENANT_B_NS}"; do
    kubectl create secret generic llm-provider-token \
      -n "${NS}" \
      --from-literal=api-key="${LLM_API_KEY}" \
      --dry-run=client -o yaml | kubectl apply -f - >/dev/null
  done
  log "  LLM secrets propagated to tenant namespaces"
fi

# ── 1. Apply RBAC isolation ──────────────────────────────────────────────────
log "Applying tenant RBAC isolation..."

# Create tenant service accounts + role bindings
for TENANT_NS in "${TENANT_A_NS}" "${TENANT_B_NS}"; do
  TENANT_LABEL="${TENANT_NS##*-}"  # e.g. "a" or "b"
  SA_NAME="agentic-operator-tenant-${TENANT_LABEL}"

  cat <<RBAC | kubectl apply -f - >/dev/null
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: ${SA_NAME}
  namespace: ${TENANT_NS}
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: agentic-tenant-role
  namespace: ${TENANT_NS}
rules:
  - apiGroups: ["agentic.clawdlinux.org"]
    resources: ["agentworkloads"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  - apiGroups: [""]
    resources: ["pods", "pods/log", "events"]
    verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: agentic-tenant-binding
  namespace: ${TENANT_NS}
subjects:
  - kind: ServiceAccount
    name: ${SA_NAME}
    namespace: ${TENANT_NS}
roleRef:
  kind: Role
  name: agentic-tenant-role
  apiGroup: rbac.authorization.k8s.io
RBAC
done
pass "Tenant RBAC applied"

# ── 2. RBAC cross-namespace denial ───────────────────────────────────────────
log "Testing RBAC cross-namespace isolation..."

SA_A="system:serviceaccount:${TENANT_A_NS}:agentic-operator-tenant-a"
SA_B="system:serviceaccount:${TENANT_B_NS}:agentic-operator-tenant-b"

# Tenant-A must NOT read tenant-B workloads
DENIED_AB="$(kubectl auth can-i list agentworkloads.agentic.clawdlinux.org \
  --namespace="${TENANT_B_NS}" --as="${SA_A}" 2>/dev/null || echo "no")"
if [[ "${DENIED_AB}" == "no" ]]; then
  pass "RBAC: tenant-A CANNOT list workloads in tenant-B"
else
  fail_test "RBAC VIOLATION: tenant-A can access tenant-B (got: ${DENIED_AB})"
fi

# Tenant-B must NOT read tenant-A workloads
DENIED_BA="$(kubectl auth can-i list agentworkloads.agentic.clawdlinux.org \
  --namespace="${TENANT_A_NS}" --as="${SA_B}" 2>/dev/null || echo "no")"
if [[ "${DENIED_BA}" == "no" ]]; then
  pass "RBAC: tenant-B CANNOT list workloads in tenant-A"
else
  fail_test "RBAC VIOLATION: tenant-B can access tenant-A (got: ${DENIED_BA})"
fi

# Tenant-A CAN manage own workloads
ALLOWED_AA="$(kubectl auth can-i update agentworkloads.agentic.clawdlinux.org \
  --namespace="${TENANT_A_NS}" --as="${SA_A}" 2>/dev/null || echo "no")"
if [[ "${ALLOWED_AA}" == "yes" ]]; then
  pass "RBAC: tenant-A CAN manage own workloads"
else
  fail_test "RBAC: tenant-A denied in own namespace (got: ${ALLOWED_AA})"
fi

echo "${DENIED_AB}" > "${EVIDENCE_DIR}/rbac_cross_ns_a_to_b.txt"
echo "${DENIED_BA}" > "${EVIDENCE_DIR}/rbac_cross_ns_b_to_a.txt"

# ── 3. Deploy workloads across namespaces ────────────────────────────────────
log "Deploying 3 workloads across 3 namespaces..."

WL_MAIN="e2e-main-${RUN_ID}"
WL_TENANT_A="e2e-tenant-a-${RUN_ID}"
WL_TENANT_B="e2e-tenant-b-${RUN_ID}"

create_workload() {
  local ns="$1" name="$2" objective="$3"
  cat <<MANIFEST | kubectl apply -f - >/dev/null
apiVersion: agentic.clawdlinux.org/v1alpha1
kind: AgentWorkload
metadata:
  name: ${name}
  namespace: ${ns}
  labels:
    e2e-run: "${RUN_ID}"
    test-type: multi-tenant
spec:
  mcpServerEndpoint: "https://httpbin.org/json"
  objective: "${objective}"
  autoApproveThreshold: "0.80"
  opaPolicy: permissive
MANIFEST
  log "  Applied ${ns}/${name}"
}

create_workload "${NS_ARGO}"     "${WL_MAIN}"     "Multi-tenant E2E: main namespace workload"
create_workload "${TENANT_A_NS}" "${WL_TENANT_A}" "Multi-tenant E2E: tenant-A isolated workload"
create_workload "${TENANT_B_NS}" "${WL_TENANT_B}" "Multi-tenant E2E: tenant-B isolated workload"
pass "3 workloads deployed"

# ── 4. Wait for all workloads to reach terminal state ────────────────────────
log "Waiting for all workloads (timeout: ${WORKLOAD_TIMEOUT}s each)..."

wait_for_workload() {
  local ns="$1" name="$2"
  local deadline=$(( $(date +%s) + WORKLOAD_TIMEOUT ))
  while true; do
    local phase
    phase="$(kubectl -n "${ns}" get agentworkload "${name}" \
      -o jsonpath='{.status.phase}' 2>/dev/null || echo "<pending>")"

    if [[ "${phase}" == "Completed" || "${phase}" == "Failed" ]]; then
      echo "${phase}"
      return 0
    fi

    if (( $(date +%s) >= deadline )); then
      echo "Timeout(${phase})"
      return 1
    fi

    sleep "${SLEEP_SECONDS}"
  done
}

PHASE_MAIN="$(wait_for_workload "${NS_ARGO}"     "${WL_MAIN}" || true)"
PHASE_A="$(wait_for_workload    "${TENANT_A_NS}" "${WL_TENANT_A}" || true)"
PHASE_B="$(wait_for_workload    "${TENANT_B_NS}" "${WL_TENANT_B}" || true)"

for ENTRY in "${NS_ARGO}/${WL_MAIN}:${PHASE_MAIN}" \
             "${TENANT_A_NS}/${WL_TENANT_A}:${PHASE_A}" \
             "${TENANT_B_NS}/${WL_TENANT_B}:${PHASE_B}"; do
  WL_FULL="${ENTRY%%:*}"
  WL_PHASE="${ENTRY##*:}"
  if [[ "${WL_PHASE}" == "Completed" || "${WL_PHASE}" == "Failed" ]]; then
    pass "${WL_FULL} reached terminal phase: ${WL_PHASE}"
  else
    fail_test "${WL_FULL} did not reach terminal phase: ${WL_PHASE}"
  fi
done

# ── 5. Verify workloads don't cross namespaces ───────────────────────────────
log "Verifying namespace isolation of workload resources..."

# Workload created in tenant-A should not appear in tenant-B
WL_LEAK_CHECK="$(kubectl -n "${TENANT_B_NS}" get agentworkload "${WL_TENANT_A}" \
  --no-headers 2>/dev/null || echo "")"
if [[ -z "${WL_LEAK_CHECK}" ]]; then
  pass "Tenant-A workload not visible in tenant-B namespace"
else
  fail_test "Tenant-A workload leaked to tenant-B namespace!"
fi

WL_LEAK_CHECK_REV="$(kubectl -n "${TENANT_A_NS}" get agentworkload "${WL_TENANT_B}" \
  --no-headers 2>/dev/null || echo "")"
if [[ -z "${WL_LEAK_CHECK_REV}" ]]; then
  pass "Tenant-B workload not visible in tenant-A namespace"
else
  fail_test "Tenant-B workload leaked to tenant-A namespace!"
fi

# ── 6. Spec round-trip per workload ──────────────────────────────────────────
log "Validating spec round-trip..."

for ENTRY in "${NS_ARGO}:${WL_MAIN}" "${TENANT_A_NS}:${WL_TENANT_A}" "${TENANT_B_NS}:${WL_TENANT_B}"; do
  NS="${ENTRY%%:*}"
  NAME="${ENTRY##*:}"
  STORED_EP="$(kubectl -n "${NS}" get agentworkload "${NAME}" \
    -o jsonpath='{.spec.mcpServerEndpoint}' 2>/dev/null || echo "")"
  if [[ "${STORED_EP}" == "https://httpbin.org/json" ]]; then
    pass "${NS}/${NAME}: mcpServerEndpoint round-trip OK"
  else
    fail_test "${NS}/${NAME}: mcpServerEndpoint mismatch: '${STORED_EP}'"
  fi
done

# ── 7. Collect evidence ──────────────────────────────────────────────────────
log "Collecting evidence..."

for ENTRY in "${NS_ARGO}:${WL_MAIN}" "${TENANT_A_NS}:${WL_TENANT_A}" "${TENANT_B_NS}:${WL_TENANT_B}"; do
  NS="${ENTRY%%:*}"
  NAME="${ENTRY##*:}"
  kubectl -n "${NS}" get agentworkload "${NAME}" -o yaml \
    > "${EVIDENCE_DIR}/agentworkload_${NAME}.yaml" 2>/dev/null || true
  kubectl -n "${NS}" describe agentworkload "${NAME}" \
    > "${EVIDENCE_DIR}/describe_${NAME}.txt" 2>/dev/null || true
  kubectl -n "${NS}" get events --sort-by=.lastTimestamp \
    > "${EVIDENCE_DIR}/events_${NS}.txt" 2>/dev/null || true
done

kubectl -n agentic-system logs deploy/agentic-operator --tail=400 \
  > "${EVIDENCE_DIR}/operator.logs.txt" 2>/dev/null || true
kubectl get nodes -o wide > "${EVIDENCE_DIR}/nodes.txt" 2>/dev/null || true

pass "Evidence collected to ${EVIDENCE_DIR}"

# ── 8. Cleanup ────────────────────────────────────────────────────────────────
if [[ "${CLEANUP}" == "true" ]]; then
  log "Cleaning up..."
  kubectl -n "${NS_ARGO}"     delete agentworkload "${WL_MAIN}"     --ignore-not-found >/dev/null 2>&1 || true
  kubectl -n "${TENANT_A_NS}" delete agentworkload "${WL_TENANT_A}" --ignore-not-found >/dev/null 2>&1 || true
  kubectl -n "${TENANT_B_NS}" delete agentworkload "${WL_TENANT_B}" --ignore-not-found >/dev/null 2>&1 || true

  # Optionally remove tenant namespaces
  for NS in "${TENANT_A_NS}" "${TENANT_B_NS}"; do
    kubectl delete namespace "${NS}" --ignore-not-found >/dev/null 2>&1 || true
  done
  pass "Cleaned up"
fi

# ── Summary ───────────────────────────────────────────────────────────────────
echo ""
echo "════════════════════════════════════════════"
echo "  Multi-Tenant E2E: ${PASS}/${TOTAL} passed, ${FAIL} failed"
echo "  Main:     ${PHASE_MAIN}"
echo "  Tenant-A: ${PHASE_A}"
echo "  Tenant-B: ${PHASE_B}"
echo "  Evidence: ${EVIDENCE_DIR}"
echo "════════════════════════════════════════════"

if (( FAIL > 0 )); then
  exit 1
fi

exit 0
