#!/usr/bin/env bash
# =============================================================================
#  teardown.sh — Clean removal of all test resources from the cluster
#
#  Safe to run multiple times. Only removes what this harness installed.
#
#  Usage:
#    export KUBECONFIG=/path/to/kubeconfig
#    bash tests/harness/teardown.sh
#
#  Options:
#    TEARDOWN_LEVEL=all       Remove everything (operator + argo + shared, default)
#    TEARDOWN_LEVEL=tests     Remove test workloads only, keep infrastructure
#    TEARDOWN_LEVEL=operator  Remove operator + test workloads, keep argo + shared
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

# Source config if present
if [[ -f "${SCRIPT_DIR}/config.env" ]]; then
  # shellcheck source=/dev/null
  source "${SCRIPT_DIR}/config.env"
fi

# Defaults
NS_ARGO="${NS_ARGO:-argo-workflows}"
NS_OPERATOR="${NS_OPERATOR:-agentic-system}"
NS_SHARED="${NS_SHARED:-shared-services}"
TEARDOWN_LEVEL="${TEARDOWN_LEVEL:-all}"

log() { echo "[teardown $(date +%H:%M:%S)] $*"; }
pass() { echo "  ✅ $*"; }

# ── 1. Delete test workloads ─────────────────────────────────────────────────
log "Deleting test AgentWorkload resources..."

# Delete all workloads with the e2e-run label in known namespaces
for NS in "${NS_ARGO}" agentic-customer-tenant-a agentic-customer-tenant-b; do
  kubectl -n "${NS}" delete agentworkloads --all --ignore-not-found >/dev/null 2>&1 || true
done
pass "Test workloads deleted"

# Delete test tenant namespaces
for NS in agentic-customer-tenant-a agentic-customer-tenant-b; do
  if kubectl get namespace "${NS}" >/dev/null 2>&1; then
    kubectl delete namespace "${NS}" --ignore-not-found >/dev/null 2>&1 || true
    log "  Deleted namespace ${NS}"
  fi
done

if [[ "${TEARDOWN_LEVEL}" == "tests" ]]; then
  log "Teardown level=tests — keeping infrastructure"
  exit 0
fi

# ── 2. Uninstall Operator ────────────────────────────────────────────────────
log "Uninstalling Agentic Operator..."

if helm list -n "${NS_OPERATOR}" 2>/dev/null | grep -q agentic-operator; then
  helm uninstall agentic-operator -n "${NS_OPERATOR}" --wait >/dev/null 2>&1 || true
  pass "Operator Helm release removed"
else
  log "  Operator Helm release not found — skipping"
fi

# Clean up CRDs (Helm does not remove CRDs on uninstall by design)
kubectl delete crd agentworkloads.agentic.clawdlinux.org --ignore-not-found >/dev/null 2>&1 || true
pass "CRDs removed"

if [[ "${TEARDOWN_LEVEL}" == "operator" ]]; then
  log "Teardown level=operator — keeping argo + shared services"
  exit 0
fi

# ── 3. Uninstall Argo Workflows ──────────────────────────────────────────────
log "Uninstalling Argo Workflows..."

if helm list -n "${NS_ARGO}" 2>/dev/null | grep -q argo-workflows; then
  helm uninstall argo-workflows -n "${NS_ARGO}" --wait >/dev/null 2>&1 || true
  pass "Argo Helm release removed"
else
  log "  Argo Helm release not found — skipping"
fi

# ── 4. Delete shared services ────────────────────────────────────────────────
log "Deleting shared services..."
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
if [[ -d "${REPO_ROOT}/config/shared-services" ]]; then
  kubectl delete -f "${REPO_ROOT}/config/shared-services/" --ignore-not-found >/dev/null 2>&1 || true
fi
pass "Shared services removed"

# ── 5. Clean up namespaces ───────────────────────────────────────────────────
log "Cleaning up namespaces..."

for NS in "${NS_OPERATOR}" "${NS_SHARED}"; do
  if kubectl get namespace "${NS}" >/dev/null 2>&1; then
    # Only delete if empty (no user workloads)
    REMAINING="$(kubectl -n "${NS}" get all --no-headers 2>/dev/null | wc -l | tr -d ' ')"
    if (( REMAINING <= 1 )); then
      kubectl delete namespace "${NS}" --ignore-not-found >/dev/null 2>&1 || true
      log "  Deleted namespace ${NS}"
    else
      log "  ⚠️  Namespace ${NS} has ${REMAINING} resources — skipping delete"
    fi
  fi
done
pass "Namespace cleanup done"

# ── Summary ───────────────────────────────────────────────────────────────────
echo ""
echo "╔══════════════════════════════════════════════════════════════════════╗"
echo "║  Teardown complete (level=${TEARDOWN_LEVEL})                                    ║"
echo "╚══════════════════════════════════════════════════════════════════════╝"
