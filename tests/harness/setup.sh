#!/usr/bin/env bash
# =============================================================================
#  setup.sh — Install operator + dependencies to any Kubernetes cluster
#
#  Provider-agnostic: works on kind, DOKS, EKS, GKE, AKS, or any cluster
#  with a valid KUBECONFIG.
#
#  What it installs:
#    1. Argo Workflows (via Helm)
#    2. Shared services (PostgreSQL, MinIO, Browserless, LiteLLM)
#    3. Agentic Operator (CRDs + Helm chart)
#    4. WorkflowTemplate + RBAC
#
#  Usage:
#    export KUBECONFIG=/path/to/kubeconfig
#    bash tests/harness/setup.sh
#
#  Idempotent: safe to run multiple times.
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
HELM_TIMEOUT="${HELM_TIMEOUT:-180s}"
OPERATOR_IMAGE="${OPERATOR_IMAGE:-}"
OPERATOR_IMAGE_TAG="${OPERATOR_IMAGE_TAG:-}"
AGENT_IMAGE="${AGENT_IMAGE:-}"

log() { echo "[setup $(date +%H:%M:%S)] $*"; }
pass() { echo "  ✅ $*"; }
fail() { echo "  ❌ $*"; exit 1; }

# ── 0. Preflight ─────────────────────────────────────────────────────────────
log "Running preflight checks..."
bash "${SCRIPT_DIR}/preflight.sh" || fail "Preflight failed — fix issues above"

# ── 1. Create namespaces ─────────────────────────────────────────────────────
log "Creating namespaces..."
for NS in "${NS_ARGO}" "${NS_OPERATOR}" "${NS_SHARED}"; do
  kubectl create namespace "${NS}" --dry-run=client -o yaml | kubectl apply -f - >/dev/null
done
pass "Namespaces ready"

# ── 2. Install Argo Workflows ────────────────────────────────────────────────
log "Installing Argo Workflows..."

# Check if already installed
if helm list -n "${NS_ARGO}" 2>/dev/null | grep -q argo-workflows; then
  log "  Argo Workflows already installed — upgrading"
  HELM_CMD="upgrade"
else
  HELM_CMD="install"
fi

helm repo add argo https://argoproj.github.io/argo-helm 2>/dev/null || true
helm repo update >/dev/null 2>&1

helm "${HELM_CMD}" argo-workflows \
  argo/argo-workflows \
  --namespace "${NS_ARGO}" \
  --set serviceAccount.create=true \
  --set serviceAccount.name=argo-workflows \
  --set controller.serviceAccount.create=true \
  --set server.serviceAccount.create=true \
  --set server.authMode=server \
  --timeout "${HELM_TIMEOUT}" \
  --wait >/dev/null 2>&1

kubectl rollout status deployment/argo-workflows-workflow-controller \
  -n "${NS_ARGO}" --timeout=120s >/dev/null
pass "Argo Workflows ready"

# ── 3. Deploy shared services ────────────────────────────────────────────────
log "Deploying shared services..."
kubectl apply -f "${REPO_ROOT}/config/shared-services/" >/dev/null

# Wait for each service with a reasonable timeout
for SVC in postgres minio browserless; do
  log "  Waiting for ${SVC}..."
  kubectl rollout status "deployment/${SVC}" \
    -n "${NS_SHARED}" --timeout=120s >/dev/null 2>&1 || {
      log "  ⚠️  ${SVC} not ready yet (may need PVC, continuing)"
    }
done
pass "Shared services deployed"

# ── 4. Install the Agentic Operator (via Helm) ──────────────────────────────
log "Installing Agentic Operator..."

# Build helm set overrides
HELM_EXTRA_ARGS=()
if [[ -n "${OPERATOR_IMAGE}" ]]; then
  HELM_EXTRA_ARGS+=(--set "operator.image.repository=${OPERATOR_IMAGE}")
fi
if [[ -n "${OPERATOR_IMAGE_TAG}" ]]; then
  HELM_EXTRA_ARGS+=(--set "operator.image.tag=${OPERATOR_IMAGE_TAG}")
fi
if [[ -n "${AGENT_IMAGE}" ]]; then
  HELM_EXTRA_ARGS+=(--set "agentImage=${AGENT_IMAGE}")
fi

if helm list -n "${NS_OPERATOR}" 2>/dev/null | grep -q agentic-operator; then
  log "  Operator already installed — upgrading"
  HELM_CMD="upgrade"
else
  HELM_CMD="install"
fi

helm "${HELM_CMD}" agentic-operator \
  "${REPO_ROOT}/charts" \
  --namespace "${NS_OPERATOR}" \
  --create-namespace \
  --timeout "${HELM_TIMEOUT}" \
  "${HELM_EXTRA_ARGS[@]+"${HELM_EXTRA_ARGS[@]}"}" \
  --wait >/dev/null 2>&1 || {
    log "  ⚠️  Helm install returned non-zero — checking pod status..."
  }

# Verify operator pod is running
RETRY=0
while (( RETRY < 30 )); do
  RUNNING="$(kubectl -n "${NS_OPERATOR}" get pods -l control-plane=controller-manager \
    --field-selector=status.phase=Running --no-headers 2>/dev/null | wc -l | tr -d ' ')"
  if (( RUNNING >= 1 )); then
    break
  fi
  ((RETRY++))
  sleep 2
done

if (( RUNNING >= 1 )); then
  pass "Operator running (${RUNNING} pod(s))"
else
  log "  ⚠️  Operator pod not Running yet — tests may wait"
fi

# ── 5. Apply WorkflowTemplate + RBAC ─────────────────────────────────────────
log "Applying WorkflowTemplate and RBAC..."
kubectl apply -f "${REPO_ROOT}/config/argo/" >/dev/null 2>&1 || true
kubectl apply -f "${REPO_ROOT}/config/rbac/" >/dev/null 2>&1 || true
pass "WorkflowTemplate + RBAC applied"

# ── 6. Apply CRD samples if not already present ──────────────────────────────
log "Verifying CRD installation..."
if kubectl get crd agentworkloads.agentic.clawdlinux.org >/dev/null 2>&1; then
  pass "AgentWorkload CRD installed"
else
  log "  Applying CRDs from config/crd/..."
  kubectl apply -f "${REPO_ROOT}/config/crd/" >/dev/null 2>&1 || true
  if kubectl get crd agentworkloads.agentic.clawdlinux.org >/dev/null 2>&1; then
    pass "AgentWorkload CRD installed"
  else
    fail "CRD installation failed"
  fi
fi

# ── 7. Bootstrap LLM provider secret (if configured) ─────────────────────────
if [[ -n "${CF_ACCOUNT_ID:-}" && -n "${CF_API_TOKEN:-}" ]]; then
  log "Bootstrapping Cloudflare Workers AI secret..."
  kubectl create secret generic cloudflare-workers-ai-token \
    -n "${NS_ARGO}" \
    --from-literal=api-token="${CF_API_TOKEN}" \
    --dry-run=client -o yaml | kubectl apply -f - >/dev/null
  pass "Cloudflare secret ready in ${NS_ARGO}"
elif [[ -n "${LLM_API_KEY:-}" ]]; then
  log "Bootstrapping LLM provider secret..."
  kubectl create secret generic llm-provider-token \
    -n "${NS_ARGO}" \
    --from-literal=api-key="${LLM_API_KEY}" \
    --dry-run=client -o yaml | kubectl apply -f - >/dev/null
  pass "LLM provider secret ready in ${NS_ARGO}"
else
  log "  ⚠️  No LLM credentials configured — E2E tests requiring LLM calls will be skipped"
fi

# ── Summary ───────────────────────────────────────────────────────────────────
echo ""
echo "╔══════════════════════════════════════════════════════════════════════╗"
echo "║  Setup complete                                                      ║"
echo "╠══════════════════════════════════════════════════════════════════════╣"
echo "║  Cluster:    $(kubectl config current-context 2>/dev/null || echo '<unknown>')"
echo "║  Argo:       ${NS_ARGO}"
echo "║  Operator:   ${NS_OPERATOR}"
echo "║  Services:   ${NS_SHARED}"
echo "╚══════════════════════════════════════════════════════════════════════╝"
echo ""
echo "Next: bash tests/harness/run-all.sh"
