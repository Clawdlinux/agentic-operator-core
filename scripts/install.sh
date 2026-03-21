#!/usr/bin/env bash
# Agentic Operator — one-command local install
# Usage: curl -sSL https://raw.githubusercontent.com/Clawdlinux/agentic-operator-core/main/scripts/install.sh | bash
set -euo pipefail

CLUSTER_NAME="${AGENTIC_CLUSTER:-agentic-operator}"
NAMESPACE="agentic-system"
REPO_URL="https://github.com/Clawdlinux/agentic-operator-core.git"

info()  { printf '\033[1;36m[agentic]\033[0m %s\n' "$*"; }
error() { printf '\033[1;31m[error]\033[0m %s\n' "$*" >&2; exit 1; }

# --- Preflight checks ---
for cmd in git kubectl helm docker kind; do
  command -v "$cmd" >/dev/null 2>&1 || error "Required tool not found: $cmd — install it first."
done

info "Cloning agentic-operator-core..."
TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT
git clone --depth 1 "$REPO_URL" "$TMPDIR/agentic-operator-core"
cd "$TMPDIR/agentic-operator-core"

info "Creating kind cluster '$CLUSTER_NAME'..."
if kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
  info "Cluster '$CLUSTER_NAME' already exists — reusing."
  kubectl cluster-info --context "kind-${CLUSTER_NAME}" >/dev/null 2>&1 || error "Cluster exists but is not reachable."
else
  kind create cluster --name "$CLUSTER_NAME" --wait 60s
fi

info "Installing AgentWorkload CRD..."
kubectl apply -f config/crd/agentworkload_crd.yaml

info "Building Helm dependencies..."
helm dependency build ./charts 2>/dev/null

info "Installing agentic-operator via Helm..."
helm upgrade --install agentic-operator ./charts \
  --namespace "$NAMESPACE" \
  --create-namespace \
  --wait --timeout 120s

info "Deploying sample workload..."
kubectl apply -f config/agentworkload_example.yaml

info ""
info "========================================="
info "  Agentic Operator is running!"
info "========================================="
info ""
info "  kubectl -n $NAMESPACE get pods"
info "  kubectl get agentworkloads -A"
info ""
info "  Docs:  https://github.com/Clawdlinux/agentic-operator-core/tree/main/docs"
info "  Site:  https://clawdlinux.org"
info ""
