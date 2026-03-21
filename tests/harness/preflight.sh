#!/usr/bin/env bash
# =============================================================================
#  preflight.sh — Provider-agnostic cluster health check
#
#  Validates that the target K8s cluster is reachable and meets minimum
#  requirements before installing the operator or running tests.
#
#  Usage:
#    export KUBECONFIG=/path/to/kubeconfig
#    bash tests/harness/preflight.sh
#
#  Exit codes:
#    0 — All checks passed
#    1 — One or more checks failed
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

# Source config if present
if [[ -f "${SCRIPT_DIR}/config.env" ]]; then
  # shellcheck source=/dev/null
  source "${SCRIPT_DIR}/config.env"
fi

# Defaults
NS_ARGO="${NS_ARGO:-argo-workflows}"
NS_OPERATOR="${NS_OPERATOR:-agentic-system}"
NS_SHARED="${NS_SHARED:-shared-services}"

PASS=0
FAIL=0
WARN=0

pass() { echo "  ✅ $*"; ((PASS++)); }
fail() { echo "  ❌ $*"; ((FAIL++)); }
warn() { echo "  ⚠️  $*"; ((WARN++)); }
log()  { echo "[preflight] $*"; }

# ── 1. CLI tools ─────────────────────────────────────────────────────────────
log "Checking required CLI tools..."

for cmd in kubectl helm; do
  if command -v "${cmd}" >/dev/null 2>&1; then
    pass "${cmd} found ($(command -v "${cmd}"))"
  else
    fail "${cmd} not found — install it first"
  fi
done

# Optional tools
for cmd in jq yq; do
  if command -v "${cmd}" >/dev/null 2>&1; then
    pass "${cmd} found (optional)"
  else
    warn "${cmd} not found (optional, evidence collection will be limited)"
  fi
done

# ── 2. Cluster connectivity ──────────────────────────────────────────────────
log "Checking cluster connectivity..."

if kubectl cluster-info >/dev/null 2>&1; then
  CONTEXT="$(kubectl config current-context 2>/dev/null || echo "<unknown>")"
  pass "Cluster reachable (context: ${CONTEXT})"
else
  fail "Cannot reach cluster — check KUBECONFIG"
  echo ""
  echo "RESULT: ${PASS} passed, ${FAIL} failed, ${WARN} warnings"
  exit 1
fi

# ── 3. K8s version ───────────────────────────────────────────────────────────
log "Checking Kubernetes version..."

SERVER_VERSION="$(kubectl version -o json 2>/dev/null | jq -r '.serverVersion.gitVersion // "unknown"' 2>/dev/null || echo "unknown")"
if [[ "${SERVER_VERSION}" == "unknown" ]]; then
  SERVER_VERSION="$(kubectl version --short 2>/dev/null | grep -i server | awk '{print $NF}' || echo "unknown")"
fi
if [[ "${SERVER_VERSION}" != "unknown" ]]; then
  pass "Server version: ${SERVER_VERSION}"
else
  warn "Could not determine server version"
fi

# ── 4. Node readiness ────────────────────────────────────────────────────────
log "Checking node readiness..."

READY_NODES="$(kubectl get nodes --no-headers 2>/dev/null | grep -c ' Ready' || echo "0")"
TOTAL_NODES="$(kubectl get nodes --no-headers 2>/dev/null | wc -l | tr -d ' ')"

if (( READY_NODES >= 1 )); then
  pass "Nodes ready: ${READY_NODES}/${TOTAL_NODES}"
else
  fail "No ready nodes found"
fi

# ── 5. CRD check (is operator already installed?) ────────────────────────────
log "Checking for AgentWorkload CRD..."

if kubectl get crd agentworkloads.agentic.clawdlinux.org >/dev/null 2>&1; then
  pass "AgentWorkload CRD exists"
else
  warn "AgentWorkload CRD not found (will be installed by setup.sh)"
fi

# ── 6. Namespace check ───────────────────────────────────────────────────────
log "Checking expected namespaces..."

for NS in "${NS_ARGO}" "${NS_OPERATOR}" "${NS_SHARED}"; do
  if kubectl get namespace "${NS}" >/dev/null 2>&1; then
    pass "Namespace ${NS} exists"
  else
    warn "Namespace ${NS} does not exist (will be created by setup.sh)"
  fi
done

# ── 7. Argo Workflows readiness ──────────────────────────────────────────────
log "Checking Argo Workflows..."

ARGO_CTRL="$(kubectl -n "${NS_ARGO}" get pods -l app.kubernetes.io/name=argo-workflows-workflow-controller \
  --field-selector=status.phase=Running --no-headers 2>/dev/null | wc -l | tr -d ' ')"

if (( ARGO_CTRL >= 1 )); then
  pass "Argo workflow controller running (${ARGO_CTRL} pod(s))"
else
  warn "Argo workflow controller not running in ${NS_ARGO} (will be installed by setup.sh)"
fi

# ── 8. Operator readiness ────────────────────────────────────────────────────
log "Checking Agentic Operator..."

OPERATOR_PODS="$(kubectl -n "${NS_OPERATOR}" get pods -l control-plane=controller-manager \
  --field-selector=status.phase=Running --no-headers 2>/dev/null | wc -l | tr -d ' ')"

if (( OPERATOR_PODS >= 1 )); then
  pass "Operator running (${OPERATOR_PODS} pod(s))"
else
  warn "Operator not running in ${NS_OPERATOR} (will be installed by setup.sh)"
fi

# ── 9. Shared services readiness ─────────────────────────────────────────────
log "Checking shared services..."

for SVC in postgres minio browserless litellm; do
  POD_COUNT="$(kubectl -n "${NS_SHARED}" get pods -l app="${SVC}" \
    --field-selector=status.phase=Running --no-headers 2>/dev/null | wc -l | tr -d ' ')"
  if (( POD_COUNT >= 1 )); then
    pass "${SVC} running in ${NS_SHARED}"
  else
    warn "${SVC} not running in ${NS_SHARED} (will be installed by setup.sh)"
  fi
done

# ── 10. Helm chart availability ──────────────────────────────────────────────
log "Checking Helm chart..."

if [[ -f "${REPO_ROOT}/charts/Chart.yaml" ]]; then
  CHART_VERSION="$(grep '^version:' "${REPO_ROOT}/charts/Chart.yaml" | awk '{print $2}')"
  pass "Helm chart found (v${CHART_VERSION})"
else
  fail "Helm chart not found at charts/Chart.yaml"
fi

# ── Summary ───────────────────────────────────────────────────────────────────
echo ""
echo "════════════════════════════════════════════"
echo "  Preflight: ${PASS} passed, ${FAIL} failed, ${WARN} warnings"
echo "════════════════════════════════════════════"

if (( FAIL > 0 )); then
  echo "Fix failures above before proceeding."
  exit 1
fi

exit 0
