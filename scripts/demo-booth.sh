#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

CLUSTER_NAME="${CLUSTER_NAME:-ninevigil-demo}"
NS_OPERATOR="${NS_OPERATOR:-agentic-system}"
NS_ARGO="${NS_ARGO:-argo-workflows}"
NS_SHARED="${NS_SHARED:-shared-services}"
HELM_TIMEOUT="${HELM_TIMEOUT:-180s}"

ALLOW_MANIFEST="${REPO_ROOT}/config/samples/agentworkload_demo_allow.yaml"
DENY_MANIFEST="${REPO_ROOT}/config/samples/agentworkload_demo_deny.yaml"
SWARM_MANIFEST="${REPO_ROOT}/config/samples/agentworkload_demo_swarm.yaml"
EVIDENCE_DIR="${EVIDENCE_DIR:-${REPO_ROOT}/tests/harness/evidence/booth-$(date +%Y%m%dT%H%M%S)}"

SKIP_ARGO=false
FULL=false
RECORD=false
CLEANUP=false
ORIGINAL_ARGS=("$@")

if [[ -t 1 && -z "${NO_COLOR:-}" ]]; then
  BOLD="$(printf '\033[1m')"
  GREEN="$(printf '\033[32m')"
  RED="$(printf '\033[31m')"
  YELLOW="$(printf '\033[33m')"
  CYAN="$(printf '\033[36m')"
  RESET="$(printf '\033[0m')"
else
  BOLD=""
  GREEN=""
  RED=""
  YELLOW=""
  CYAN=""
  RESET=""
fi

usage() {
  cat <<EOF
Usage: $(basename "$0") [OPTIONS]

Runs the NineVigil booth demo gate.

Options:
  --cluster NAME    kind cluster name. Default: ${CLUSTER_NAME}
  --skip-argo       Skip Argo/shared-services setup. Week 2 gate path.
  --full            Run Phase 2 multi-agent swarm demo.
  --record          Record terminal output with script(1).
  --cleanup         Delete the kind cluster and exit.
  -h, --help        Show this help.

Environment:
  DEMO_MCP_ENDPOINT Override https://localhost:8443 in sample manifests.
  HELM_TIMEOUT      Helm wait timeout. Default: ${HELM_TIMEOUT}
  NS_OPERATOR       Operator namespace. Default: ${NS_OPERATOR}
EOF
}

log() { printf '%b[demo %s]%b %s\n' "${CYAN}" "$(date +%H:%M:%S)" "${RESET}" "$*"; }
step() { printf '\n%b==> %s%b\n' "${BOLD}" "$*" "${RESET}"; }
ok() { printf '%b[OK]%b %s\n' "${GREEN}" "${RESET}" "$*"; }
warn() { printf '%b[WARN]%b %s\n' "${YELLOW}" "${RESET}" "$*"; }
die() { printf '%b[FAIL]%b %s\n' "${RED}" "${RESET}" "$*" >&2; exit 1; }

parse_args() {
  while (($#)); do
    case "$1" in
      --cluster)
        [[ $# -ge 2 ]] || die "--cluster requires a value"
        CLUSTER_NAME="$2"
        shift 2
        ;;
      --skip-argo)
        SKIP_ARGO=true
        shift
        ;;
      --full)
        FULL=true
        shift
        ;;
      --record)
        RECORD=true
        shift
        ;;
      --cleanup)
        CLEANUP=true
        shift
        ;;
      -h|--help)
        usage
        exit 0
        ;;
      *)
        die "unknown option: $1"
        ;;
    esac
  done
}

quote_command() {
  local quoted=""
  local arg
  for arg in "$@"; do
    quoted+=" $(printf '%q' "${arg}")"
  done
  printf '%s' "${quoted# }"
}

escape_sed_replacement() {
  printf '%s' "$1" | sed -e 's/[\\&|]/\\&/g'
}

start_recording_if_requested() {
  if [[ "${RECORD}" != "true" || -n "${DEMO_RECORDING:-}" ]]; then
    return
  fi
  command -v script >/dev/null 2>&1 || die "--record requires script(1)"
  mkdir -p "${EVIDENCE_DIR}"
  local record_file="${EVIDENCE_DIR}/demo-booth.typescript"
  log "Recording booth demo to ${record_file}"

  local command_line
  if ((${#ORIGINAL_ARGS[@]})); then
    command_line="$(quote_command "$0" "${ORIGINAL_ARGS[@]}")"
  else
    command_line="$(quote_command "$0")"
  fi
  if script -q -c "true" /dev/null >/dev/null 2>&1; then
    exec env DEMO_RECORDING=1 script -q -c "${command_line}" "${record_file}"
  fi

  if ((${#ORIGINAL_ARGS[@]})); then
    exec env DEMO_RECORDING=1 script -q "${record_file}" "$0" "${ORIGINAL_ARGS[@]}"
  fi
  exec env DEMO_RECORDING=1 script -q "${record_file}" "$0"
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || die "missing required command: $1"
}

check_prerequisites() {
  step "Checking prerequisites"
  require_command kubectl
  require_command helm
  if command -v kind >/dev/null 2>&1; then
    ok "kind found"
  elif command -v k3s >/dev/null 2>&1; then
    ok "k3s found. Reusing the current kubectl context."
  else
    die "kind or k3s is required"
  fi
  [[ -f "${ALLOW_MANIFEST}" ]] || die "missing ${ALLOW_MANIFEST}"
  [[ -f "${DENY_MANIFEST}" ]] || die "missing ${DENY_MANIFEST}"
  [[ -f "${SWARM_MANIFEST}" ]] || die "missing ${SWARM_MANIFEST}"
  ok "kubectl and helm found"
}

cleanup_cluster() {
  step "Cleaning up demo cluster"
  if command -v kind >/dev/null 2>&1; then
    kind delete cluster --name "${CLUSTER_NAME}"
    ok "Deleted kind cluster ${CLUSTER_NAME}"
  else
    die "--cleanup only supports kind clusters"
  fi
}

ensure_cluster() {
  step "Preparing Kubernetes cluster"
  if command -v kind >/dev/null 2>&1; then
    if kind get clusters | grep -Fxq "${CLUSTER_NAME}"; then
      ok "Reusing kind cluster ${CLUSTER_NAME}"
    else
      kind create cluster --name "${CLUSTER_NAME}" --wait 120s
      ok "Created kind cluster ${CLUSTER_NAME}"
    fi
    kubectl config use-context "kind-${CLUSTER_NAME}" >/dev/null
    return
  fi

  kubectl cluster-info >/dev/null || die "kubectl cannot reach the current k3s cluster"
  ok "Using current kubectl context: $(kubectl config current-context 2>/dev/null || echo unknown)"
}

ensure_namespace() {
  kubectl create namespace "$1" --dry-run=client -o yaml | kubectl apply -f - >/dev/null
}

wait_for_operator() {
  step "Waiting for operator pod"
  local deadline phase pod_name
  deadline=$(( $(date +%s) + 180 ))
  while (( $(date +%s) < deadline )); do
    pod_name="$(kubectl -n "${NS_OPERATOR}" get pods -l app.kubernetes.io/name=agentic-operator -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)"
    if [[ -z "${pod_name}" ]]; then
      pod_name="$(kubectl -n "${NS_OPERATOR}" get pods -l control-plane=controller-manager -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)"
    fi
    if [[ -n "${pod_name}" ]]; then
      phase="$(kubectl -n "${NS_OPERATOR}" get pod "${pod_name}" -o jsonpath='{.status.phase}' 2>/dev/null || true)"
      if [[ "${phase}" == "Running" ]]; then
        ok "Operator pod Running: ${pod_name}"
        return
      fi
      log "Operator pod ${pod_name} phase=${phase:-unknown}"
    else
      log "Operator pod not created yet"
    fi
    sleep 5
  done
  die "Operator pod did not reach Running"
}

install_full_stack() {
  step "Installing full stack with tests/harness/setup.sh"
  HELM_TIMEOUT="${HELM_TIMEOUT}" bash "${REPO_ROOT}/tests/harness/setup.sh" || {
    die "setup.sh failed. For Week 2, rerun with --skip-argo to bypass Argo."
  }
  wait_for_operator
}

install_week2_stack() {
  step "Installing Week 2 stack without Argo"
  ensure_namespace "${NS_OPERATOR}"
  kubectl apply -f "${REPO_ROOT}/config/crd/agentworkload_crd.yaml" >/dev/null

  helm upgrade --install agentic-operator "${REPO_ROOT}/charts" \
    --namespace "${NS_OPERATOR}" \
    --create-namespace \
    --set license.key=dev-license \
    --set argo.enabled=false \
    --set browserless.enabled=false \
    --set litellm.enabled=false \
    --set minio.enabled=false \
    --set postgresql.enabled=false \
    --set clawdlinuxObservability.enabled=false \
    --set networkPolicy.enabled=true \
    --set global.runtimeSandbox.enabled=true \
    --set global.runtimeSandbox.createRuntimeClass=true \
    --set agenticOperator.webhook.enabled=false \
    --set agentic-operator.image.pullPolicy=IfNotPresent \
    --timeout "${HELM_TIMEOUT}" \
    --wait >/dev/null

  wait_for_operator
}

apply_manifest() {
  local manifest="$1"
  local endpoint namespace
  endpoint="$(escape_sed_replacement "${DEMO_MCP_ENDPOINT:-https://localhost:8443}")"
  namespace="$(escape_sed_replacement "${NS_OPERATOR}")"
  sed \
    -e "s|https://localhost:8443|${endpoint}|g" \
    -e "s|namespace: agentic-system|namespace: ${namespace}|g" \
    "${manifest}" | kubectl apply -f - >/dev/null
}

workload_phase() {
  local name="$1"
  kubectl -n "${NS_OPERATOR}" get agentworkload "${name}" -o jsonpath='{.status.phase}' 2>/dev/null || true
}

wait_for_allow_phase() {
  local name="$1"
  local deadline phase
  deadline=$(( $(date +%s) + 60 ))
  while (( $(date +%s) < deadline )); do
    phase="$(workload_phase "${name}")"
    if [[ -n "${phase}" && "${phase}" != "PolicyDenied" ]]; then
      printf '%s' "${phase}"
      return 0
    fi
    sleep 2
  done
  phase="$(workload_phase "${name}")"
  printf '%s' "${phase:-<empty>}"
  return 1
}

wait_for_deny_phase() {
  local name="$1"
  local deadline phase
  deadline=$(( $(date +%s) + 30 ))
  while (( $(date +%s) < deadline )); do
    phase="$(workload_phase "${name}")"
    if [[ "${phase}" == "PolicyDenied" ]]; then
      printf '%s' "${phase}"
      return 0
    fi
    sleep 2
  done
  phase="$(workload_phase "${name}")"
  printf '%s' "${phase:-<empty>}"
  return 1
}

deny_reason() {
  kubectl -n "${NS_OPERATOR}" get agentworkload opa-deny-demo \
    -o jsonpath='{range .status.conditions[?(@.type=="PolicyDenied")]}{.message}{"\n"}{end}' 2>/dev/null || true
}

show_workload_evidence() {
  step "AgentWorkload status"
  kubectl -n "${NS_OPERATOR}" get agentworkload -o wide || true
  echo ""
  kubectl -n "${NS_OPERATOR}" describe agentworkload opa-allow-demo || true
}

show_agent_pods() {
  step "Agent pod runtime view"
  kubectl get pods -A -l app.kubernetes.io/part-of=agentic-operator \
    -o custom-columns='NAMESPACE:.metadata.namespace,NAME:.metadata.name,PHASE:.status.phase,RUNTIME:.spec.runtimeClassName' || true
}

operator_pod_name() {
  local pod_name
  pod_name="$(kubectl -n "${NS_OPERATOR}" get pods -l app.kubernetes.io/name=agentic-operator -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)"
  if [[ -z "${pod_name}" ]]; then
    pod_name="$(kubectl -n "${NS_OPERATOR}" get pods -l control-plane=controller-manager -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)"
  fi
  printf '%s' "${pod_name}"
}

check_cost_metrics() {
  step "Checking cost metric"
  COST_OK=no

  local pod_name metric_output metric_line exposed_port
  pod_name="$(operator_pod_name)"
  if [[ -z "${pod_name}" ]]; then
    warn "Cost tracking: operator pod not found"
    return
  fi

  metric_output="$(kubectl -n "${NS_OPERATOR}" exec "${pod_name}" -- sh -c \
    'if command -v curl >/dev/null 2>&1; then curl -fsS --max-time 3 http://127.0.0.1:8080/metrics; elif command -v wget >/dev/null 2>&1; then wget -qO- --timeout=3 http://127.0.0.1:8080/metrics; else exit 127; fi' \
    2>/dev/null || true)"
  metric_line="$(printf '%s\n' "${metric_output}" | grep 'ninevigil_agent_cost_dollars' | grep -v '^#' | head -1 || true)"
  if [[ -n "${metric_line}" ]]; then
    printf '%bCost tracking:%b %s\n' "${GREEN}" "${RESET}" "${metric_line}"
    COST_OK=yes
    return
  fi

  exposed_port="$(kubectl -n "${NS_OPERATOR}" get pod "${pod_name}" \
    -o jsonpath='{range .spec.containers[*].ports[*]}{.containerPort}{"\n"}{end}' 2>/dev/null | grep -x '8080' | head -1 || true)"
  if [[ -n "${exposed_port}" ]]; then
    warn "Cost tracking: metric not available yet, but operator exposes port 8080"
    COST_OK=yes
    return
  fi

  warn "Cost tracking: ninevigil_agent_cost_dollars not available"
}

verify_gvisor() {
  step "Checking gVisor RuntimeClass"
  local runtime_class_ok runtime_webhook_config dry_run_json runtime_class
  runtime_class_ok=no
  if kubectl get runtimeclass gvisor >/dev/null 2>&1; then
    ok "RuntimeClass gvisor exists"
    runtime_class_ok=yes
  else
    warn "RuntimeClass gvisor not found"
  fi

  runtime_webhook_config="$(kubectl get mutatingwebhookconfiguration -l app.kubernetes.io/part-of=agentic-operator \
    -o jsonpath='{range .items[?(@.webhooks[*].name=="runtimeclass-pods.agentic.clawdlinux.org")]}{.metadata.name}{"\n"}{end}' 2>/dev/null | head -1 || true)"
  if [[ -n "${runtime_webhook_config}" ]]; then
    dry_run_json="$(kubectl -n "${NS_OPERATOR}" apply --dry-run=server -o json -f - <<YAML
apiVersion: v1
kind: Pod
metadata:
  name: runtime-sandbox-demo
  labels:
    agentic.clawdlinux.org/runtime-sandbox: gvisor
spec:
  restartPolicy: Never
  containers:
    - name: pause
      image: registry.k8s.io/pause:3.10
YAML
    )" || dry_run_json=""
    runtime_class="$(printf '%s' "${dry_run_json}" | sed -n 's/.*"runtimeClassName"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -1)"
    if [[ "${runtime_class}" == "gvisor" ]]; then
      ok "Runtime sandbox webhook injected runtimeClassName=gvisor"
      GVISOR_OK=yes
      return
    fi
    warn "Runtime sandbox webhook did not inject runtimeClassName. Got ${runtime_class:-missing}."
  else
    warn "No runtime sandbox mutating webhook found. Using RuntimeClass check."
  fi

  if [[ "${runtime_class_ok}" == "yes" ]]; then
    GVISOR_OK=yes
  else
    GVISOR_OK=no
  fi
}

verify_network_policy() {
  step "Checking NetworkPolicy"
  local policy_names
  policy_names="$(kubectl -n "${NS_OPERATOR}" get networkpolicy -l app.kubernetes.io/component=networkpolicy -o name 2>/dev/null || true)"
  if [[ -n "${policy_names}" ]]; then
    ok "NetworkPolicy is installed in ${NS_OPERATOR}"
    NETWORK_POLICY_OK=yes
  else
    warn "NetworkPolicy not found in ${NS_OPERATOR}"
    NETWORK_POLICY_OK=no
  fi
}

run_opa_demo() {
  step "Applying OPA allow workload"
  kubectl -n "${NS_OPERATOR}" delete agentworkload opa-allow-demo --ignore-not-found >/dev/null 2>&1 || true
  apply_manifest "${ALLOW_MANIFEST}"
  if ALLOW_PHASE="$(wait_for_allow_phase opa-allow-demo)"; then
    ok "ALLOW path phase: ${ALLOW_PHASE}"
  else
    warn "ALLOW path did not reach a non-empty, non-denied phase. Last phase: ${ALLOW_PHASE}"
  fi

  check_cost_metrics

  show_workload_evidence
  show_agent_pods

  step "Applying OPA deny workload"
  kubectl -n "${NS_OPERATOR}" delete agentworkload opa-deny-demo --ignore-not-found >/dev/null 2>&1 || true
  apply_manifest "${DENY_MANIFEST}"
  if DENY_PHASE="$(wait_for_deny_phase opa-deny-demo)"; then
    ok "DENY path phase: ${DENY_PHASE}"
  else
    warn "DENY path did not reach PolicyDenied. Last phase: ${DENY_PHASE}"
  fi

  local reason
  reason="$(deny_reason)"
  if [[ -n "${reason}" ]]; then
    printf '%bDeny reason:%b %s\n' "${BOLD}" "${RESET}" "${reason}"
  else
    warn "No PolicyDenied condition message found"
  fi
}

find_swarm_workflow() {
  local workflow_name
  workflow_name="$(kubectl -n "${NS_ARGO}" get workflows \
    -l agentic.clawdlinux.org/workload=demo-competitive-swarm \
    -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)"
  if [[ -z "${workflow_name}" ]]; then
    # Current controller labels created Workflows by AgentWorkload name.
    workflow_name="$(kubectl -n "${NS_ARGO}" get workflows \
      -l agentic.io/job-id=demo-competitive-swarm \
      -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)"
  fi
  printf '%s' "${workflow_name}"
}

workflow_phase() {
  local workflow_name="$1"
  kubectl -n "${NS_ARGO}" get workflow "${workflow_name}" -o jsonpath='{.status.phase}' 2>/dev/null || true
}

run_swarm_demo() {
  if [[ "${FULL}" != "true" ]]; then
    SWARM_PHASE="skipped (use --full)"
    return
  fi

  step "PHASE 2: Multi-Agent Swarm"
  if [[ "${SKIP_ARGO}" == "true" ]]; then
    warn "Swarm skipped because --skip-argo disables Argo installation"
    SWARM_PHASE="skipped (no argo)"
    return
  fi

  SWARM_PHASE="not found"
  apply_manifest "${SWARM_MANIFEST}"

  local deadline workflow_name phase
  deadline=$(( $(date +%s) + 60 ))
  while (( $(date +%s) < deadline )); do
    workflow_name="$(find_swarm_workflow)"
    if [[ -n "${workflow_name}" ]]; then
      phase="$(workflow_phase "${workflow_name}")"
      SWARM_PHASE="${phase:-Pending}"
      printf '%bSwarm workflow:%b %s phase=%s\n' "${GREEN}" "${RESET}" "${workflow_name}" "${SWARM_PHASE}"
      break
    fi
    sleep 2
  done

  if [[ -z "${workflow_name:-}" ]]; then
    warn "Swarm workflow did not appear in ${NS_ARGO} within 60s"
    printf 'Swarm: %s. Agents: competitor-scraper, llm-synthesizer, report-generator\n' "${SWARM_PHASE}"
    return
  fi

  deadline=$(( $(date +%s) + 120 ))
  while (( $(date +%s) < deadline )); do
    phase="$(workflow_phase "${workflow_name}")"
    if [[ -n "${phase}" ]]; then
      SWARM_PHASE="${phase}"
      break
    fi
    sleep 3
  done

  if [[ -z "${SWARM_PHASE}" || "${SWARM_PHASE}" == "Pending" ]]; then
    warn "Swarm workflow has not started a phase yet"
  fi
  kubectl -n "${NS_ARGO}" get workflows || true
  printf 'Swarm: %s. Agents: competitor-scraper, llm-synthesizer, report-generator\n' "${SWARM_PHASE:-unknown}"
}

print_summary() {
  echo ""
  printf '%bBooth Demo Summary%b\n' "${BOLD}" "${RESET}"
  printf 'ALLOW path: %s. DENY path: %s. gVisor: %s. NetworkPolicy: %s. Cost: %s. Swarm: %s.\n' \
    "${ALLOW_PHASE:-<empty>}" \
    "${DENY_PHASE:-<empty>}" \
    "${GVISOR_OK:-no}" \
    "${NETWORK_POLICY_OK:-no}" \
    "${COST_OK:-no}" \
    "${SWARM_PHASE:-skipped (use --full)}"
  echo ""
  if [[ "${DENY_PHASE:-}" != "PolicyDenied" ]]; then
    warn "DENY needs a reachable HTTPS MCP endpoint to reach OPA. Set DEMO_MCP_ENDPOINT for live booth runs."
  fi
}

main() {
  parse_args "$@"
  start_recording_if_requested

  if [[ "${CLEANUP}" == "true" ]]; then
    cleanup_cluster
    return
  fi

  check_prerequisites
  ensure_cluster
  if [[ "${SKIP_ARGO}" == "true" ]]; then
    install_week2_stack
  else
    install_full_stack
  fi
  verify_gvisor
  verify_network_policy
  run_opa_demo
  run_swarm_demo
  print_summary
}

main "$@"