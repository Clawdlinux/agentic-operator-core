#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

CLUSTER_NAME="${CLUSTER_NAME:-clawdlinux-demo}"
NS_OPERATOR="${NS_OPERATOR:-agentic-system}"
NS_ARGO="${NS_ARGO:-argo-workflows}"
NS_SHARED="${NS_SHARED:-shared-services}"
HELM_TIMEOUT="${HELM_TIMEOUT:-180s}"
OPERATOR_IMAGE="${OPERATOR_IMAGE:-ghcr.io/clawdlinux/agentic-operator-core/agentic-operator}"
OPERATOR_IMAGE_TAG="${OPERATOR_IMAGE_TAG:-latest}"
CERT_MANAGER_VERSION="${CERT_MANAGER_VERSION:-v1.17.2}"
DEMO_RELEASE="${DEMO_RELEASE:-clawdlinux-demo}"
DEMO_SECRET="${DEMO_SECRET:-clawdlinux-demo-litellm}"
DEMO_ENV_FILE="${DEMO_ENV_FILE:-${REPO_ROOT}/.env}"
RESEARCH_MANIFEST="${REPO_ROOT}/examples/research-agent.yaml"
AUDIT_FIXTURE="${REPO_ROOT}/_staging/booth/attestation-fallback.jsonl"
# Committed demo fixture key. Never use it as a production secret.
AUDIT_DEMO_KEY="booth-demo-2026=bmluZXZpZ2lsLWJvb3RoLWRlbW8tYXR0ZXN0YXRpb24ta2V5LTMyYg=="

ALLOW_MANIFEST="${REPO_ROOT}/config/samples/agentworkload_demo_allow.yaml"
DENY_MANIFEST="${REPO_ROOT}/config/samples/agentworkload_demo_deny.yaml"
SWARM_MANIFEST="${REPO_ROOT}/config/samples/agentworkload_demo_swarm.yaml"
EVIDENCE_DIR="${EVIDENCE_DIR:-${REPO_ROOT}/tests/harness/evidence/booth-$(date +%Y%m%dT%H%M%S)}"

DEMO_PROFILE="${DEMO_PROFILE:-platform}"
DEMO_STAGE_DELAY_SECONDS="${DEMO_STAGE_DELAY_SECONDS:-6}"
WITH_SWARM=false
RECORD=false
CLEANUP=false
DEMO_MODE=legacy
TAMPER_AUDIT=false
PORT_FORWARD_PID=""
ORIGINAL_ARGS=("$@")

if [[ ( -t 1 || "${FORCE_COLOR:-}" == "1" ) && -z "${NO_COLOR:-}" ]]; then
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

Runs the Clawdlinux booth demo gate.

Options:
  --prepare         Create and prepare the real-provider kind demo cluster.
  --present         Run the 5-7 minute real-provider booth presentation.
  --tamper-audit    Tamper with the prior-run audit fixture and prove rejection.
  --pace SECONDS    Pause between evidence stages. Default: ${DEMO_STAGE_DELAY_SECONDS}
  --cluster NAME    kind cluster name. Default: ${CLUSTER_NAME}
  --profile NAME    Deployment profile: platform or lean. Default: ${DEMO_PROFILE}
  --with-swarm      Run the optional Argo multi-agent swarm scenario.
  --skip-argo       Compatibility alias for --profile lean.
  --full            Compatibility alias for --with-swarm.
  --record          Record terminal output with script(1).
  --cleanup         Delete the kind cluster and exit.
  -h, --help        Show this help.

Environment:
  DEMO_MCP_ENDPOINT Override https://localhost:8443 in sample manifests.
  HELM_TIMEOUT      Helm wait timeout. Default: ${HELM_TIMEOUT}
  NS_OPERATOR       Operator namespace. Default: ${NS_OPERATOR}
  OPERATOR_IMAGE    Lean-profile operator image repository. Default: ${OPERATOR_IMAGE}
  OPERATOR_IMAGE_TAG Lean-profile operator image tag. Default: ${OPERATOR_IMAGE_TAG}
  CERT_MANAGER_VERSION Pinned cert-manager chart version. Default: ${CERT_MANAGER_VERSION}
  DEMO_ENV_FILE     Credential file. Default: ${REPO_ROOT}/.env
  DEMO_STAGE_DELAY_SECONDS Narration delay. Default: 6
  FORCE_COLOR       Set to 1 to preserve ANSI colors through a recorder or pipe.
EOF
}

log() { printf '%b[demo %s]%b %s\n' "${CYAN}" "$(date +%H:%M:%S)" "${RESET}" "$*"; }
step() { printf '\n%b==> %s%b\n' "${BOLD}" "$*" "${RESET}"; }
ok() { printf '%b[OK]%b %s\n' "${GREEN}" "${RESET}" "$*"; }
warn() { printf '%b[WARN]%b %s\n' "${YELLOW}" "${RESET}" "$*"; }
die() { printf '%b[FAIL]%b %s\n' "${RED}" "${RESET}" "$*" >&2; exit 1; }

validate_pace() {
  [[ "$1" =~ ^([0-9]|[1-5][0-9]|60)$ ]] || die "pace must be an integer from 0 to 60"
}

narration_pause() {
  validate_pace "${DEMO_STAGE_DELAY_SECONDS}"
  printf 'Narration pause: %ss\n' "${DEMO_STAGE_DELAY_SECONDS}"
  if ((10#${DEMO_STAGE_DELAY_SECONDS} > 0)); then
    sleep "${DEMO_STAGE_DELAY_SECONDS}"
  fi
}

parse_args() {
  while (($#)); do
    case "$1" in
      --prepare)
        [[ "${DEMO_MODE}" == "legacy" || "${DEMO_MODE}" == "prepare" ]] || die "--prepare and --present are mutually exclusive"
        DEMO_MODE=prepare
        shift
        ;;
      --present)
        [[ "${DEMO_MODE}" == "legacy" || "${DEMO_MODE}" == "present" ]] || die "--prepare and --present are mutually exclusive"
        DEMO_MODE=present
        shift
        ;;
      --tamper-audit)
        TAMPER_AUDIT=true
        shift
        ;;
      --pace)
        [[ $# -ge 2 ]] || die "--pace requires a value"
        validate_pace "$2"
        DEMO_STAGE_DELAY_SECONDS="$2"
        shift 2
        ;;
      --cluster)
        [[ $# -ge 2 ]] || die "--cluster requires a value"
        CLUSTER_NAME="$2"
        shift 2
        ;;
      --profile)
        [[ $# -ge 2 ]] || die "--profile requires platform or lean"
        case "$2" in
          platform|lean)
            DEMO_PROFILE="$2"
            ;;
          *)
            die "invalid profile: $2 (expected platform or lean)"
            ;;
        esac
        shift 2
        ;;
      --skip-argo)
        DEMO_PROFILE=lean
        shift
        ;;
      --with-swarm|--full)
        WITH_SWARM=true
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
  validate_pace "${DEMO_STAGE_DELAY_SECONDS}"
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

trim_horizontal_space() {
  local value="$1"
  value="${value#"${value%%[![:space:]]*}"}"
  value="${value%"${value##*[![:space:]]}"}"
  printf '%s' "${value}"
}

parse_demo_credential_value() {
  local key="$1"
  local raw_value="$2"
  local source_file="$3"
  local line_number="$4"
  local value
  value="$(trim_horizontal_space "${raw_value}")"

  if [[ "${value}" == *$'\n'* || "${value}" == *$'\r'* || "${value}" == *'\' ]]; then
    die "malformed definition for ${key} in ${source_file}:${line_number}"
  fi
  if [[ "${value}" == "'"* ]]; then
    [[ ${#value} -ge 2 && "${value: -1}" == "'" ]] || die "malformed definition for ${key} in ${source_file}:${line_number}"
    value="${value:1:${#value}-2}"
  elif [[ "${value}" == '"'* ]]; then
    [[ ${#value} -ge 2 && "${value: -1}" == '"' ]] || die "malformed definition for ${key} in ${source_file}:${line_number}"
    value="${value:1:${#value}-2}"
  fi

  printf '%s' "${value}"
}

reject_credential_newlines() {
  local key="$1"
  local value="$2"
  local source="$3"
  if [[ "${value}" == *$'\n'* || "${value}" == *$'\r'* ]]; then
    die "unsafe newline in ${key} from ${source}"
  fi
}

valid_litellm_master_key() {
  local value="$1"
  [[ "${value}" == sk-* && ${#value} -ge 19 && "${value}" != *$'\n'* && "${value}" != *$'\r'* ]]
}

decode_base64() {
  if printf '' | base64 --decode >/dev/null 2>&1; then
    base64 --decode
  elif printf '' | base64 -D >/dev/null 2>&1; then
    base64 -D
  else
    return 1
  fi
}

load_demo_credentials() {
  local openai_from_environment=false
  local anthropic_from_environment=false
  local openai_definitions=0
  local anthropic_definitions=0
  local line line_number=0 key raw_value parsed_value

  if [[ ${OPENAI_API_KEY+x} == x ]]; then
    reject_credential_newlines "OPENAI_API_KEY" "${OPENAI_API_KEY}" "environment"
    openai_from_environment=true
    OPENAI_API_KEY_SOURCE="environment"
  fi
  if [[ ${ANTHROPIC_API_KEY+x} == x ]]; then
    reject_credential_newlines "ANTHROPIC_API_KEY" "${ANTHROPIC_API_KEY}" "environment"
    anthropic_from_environment=true
    ANTHROPIC_API_KEY_SOURCE="environment"
  fi

  if [[ ! -e "${DEMO_ENV_FILE}" ]]; then
    return 0
  fi
  [[ -f "${DEMO_ENV_FILE}" && -r "${DEMO_ENV_FILE}" ]] || die "credential file is not readable: ${DEMO_ENV_FILE}"

  while IFS= read -r line || [[ -n "${line}" ]]; do
    ((line_number += 1))
    if [[ "${line}" == *$'\r' ]]; then
      line="${line%$'\r'}"
    fi

    if [[ "${line}" =~ ^[[:space:]]*(export[[:space:]]+)?(OPENAI_API_KEY|ANTHROPIC_API_KEY)([[:space:]]|=|$) ]]; then
      key="${BASH_REMATCH[2]}"
    else
      continue
    fi

    case "${key}" in
      OPENAI_API_KEY)
        ((openai_definitions += 1))
        ((openai_definitions == 1)) || die "duplicate definition for OPENAI_API_KEY in ${DEMO_ENV_FILE}"
        ;;
      ANTHROPIC_API_KEY)
        ((anthropic_definitions += 1))
        ((anthropic_definitions == 1)) || die "duplicate definition for ANTHROPIC_API_KEY in ${DEMO_ENV_FILE}"
        ;;
    esac

    if [[ "${line}" =~ ^[[:space:]]*(export[[:space:]]+)?(OPENAI_API_KEY|ANTHROPIC_API_KEY)[[:space:]]*=(.*)$ ]]; then
      raw_value="${BASH_REMATCH[3]}"
    else
      die "malformed definition for ${key} in ${DEMO_ENV_FILE}:${line_number}"
    fi
    parsed_value="$(parse_demo_credential_value "${key}" "${raw_value}" "${DEMO_ENV_FILE}" "${line_number}")"

    case "${key}" in
      OPENAI_API_KEY)
        if [[ "${openai_from_environment}" != "true" ]]; then
          OPENAI_API_KEY="${parsed_value}"
          export OPENAI_API_KEY
          OPENAI_API_KEY_SOURCE="${DEMO_ENV_FILE}"
        fi
        ;;
      ANTHROPIC_API_KEY)
        if [[ "${anthropic_from_environment}" != "true" ]]; then
          ANTHROPIC_API_KEY="${parsed_value}"
          export ANTHROPIC_API_KEY
          ANTHROPIC_API_KEY_SOURCE="${DEMO_ENV_FILE}"
        fi
        ;;
    esac
  done <"${DEMO_ENV_FILE}"
}

require_real_provider_keys() {
  local missing=()
  load_demo_credentials
  [[ -n "${ANTHROPIC_API_KEY:-}" ]] || missing+=(ANTHROPIC_API_KEY)
  if ((${#missing[@]})); then
    printf '%b[FAIL]%b Real-provider preparation requires: %s\n' "${RED}" "${RESET}" "${missing[*]}" >&2
    printf 'Export the missing values or define them in %s, then rerun.\n' "${DEMO_ENV_FILE}" >&2
    exit 1
  fi
  printf 'ANTHROPIC_API_KEY=available\n'
  printf 'Credential variable ANTHROPIC_API_KEY loaded from %s\n' "${ANTHROPIC_API_KEY_SOURCE}"
}

require_demo_context() {
  local expected_context="kind-${CLUSTER_NAME}"
  local current_context
  current_context="$(kubectl config current-context 2>/dev/null || true)"
  [[ "${current_context}" == "${expected_context}" ]] || {
    die "wrong kubectl context: ${current_context:-none}. Expected ${expected_context}. Run --prepare first."
  }
}

ensure_demo_kind_cluster() {
  step "Preparing kind cluster ${CLUSTER_NAME}"
  require_command kind
  if kind get clusters | grep -Fxq "${CLUSTER_NAME}"; then
    ok "Reusing kind cluster ${CLUSTER_NAME}"
  else
    kind create cluster --name "${CLUSTER_NAME}" --wait 120s
    ok "Created kind cluster ${CLUSTER_NAME}"
  fi
  kubectl config use-context "kind-${CLUSTER_NAME}" >/dev/null
  require_demo_context
}

install_cert_manager() {
  step "Installing cert-manager ${CERT_MANAGER_VERSION}"
  helm upgrade --install cert-manager oci://quay.io/jetstack/charts/cert-manager \
    --namespace cert-manager \
    --create-namespace \
    --version "${CERT_MANAGER_VERSION}" \
    --set crds.enabled=true \
    --timeout "${HELM_TIMEOUT}" \
    --wait >/dev/null
  kubectl -n cert-manager rollout status deployment/cert-manager --timeout="${HELM_TIMEOUT}" >/dev/null
  kubectl -n cert-manager rollout status deployment/cert-manager-webhook --timeout="${HELM_TIMEOUT}" >/dev/null
  kubectl -n cert-manager rollout status deployment/cert-manager-cainjector --timeout="${HELM_TIMEOUT}" >/dev/null
  ok "cert-manager is ready"
}

create_runtime_provider_secret() {
  step "Creating runtime provider Secret"
  local existing_master_key=""
  local existing_master_key_with_sentinel=""
  local master_key
  if [[ -n "${LITELLM_MASTER_KEY:-}" ]]; then
    reject_credential_newlines "LITELLM_MASTER_KEY" "${LITELLM_MASTER_KEY}" "environment"
    valid_litellm_master_key "${LITELLM_MASTER_KEY}" ||
      die "LITELLM_MASTER_KEY must start with sk- and contain at least 16 key characters"
    master_key="${LITELLM_MASTER_KEY}"
  else
    existing_master_key_with_sentinel="$({
      kubectl -n "${NS_OPERATOR}" get secret "${DEMO_SECRET}" \
        -o jsonpath='{.data.LITELLM_MASTER_KEY}' 2>/dev/null | decode_base64 2>/dev/null || true
      printf '.'
    })"
    existing_master_key="${existing_master_key_with_sentinel%.}"
    if valid_litellm_master_key "${existing_master_key}"; then
      master_key="${existing_master_key}"
    else
      require_command openssl
      master_key="sk-$(openssl rand -hex 24)"
    fi
  fi

  {
    printf 'ANTHROPIC_API_KEY=%s\n' "${ANTHROPIC_API_KEY}"
    printf 'LITELLM_MASTER_KEY=%s\n' "${master_key}"
    printf 'api-key=%s\n' "${master_key}"
  } | kubectl -n "${NS_OPERATOR}" create secret generic "${DEMO_SECRET}" \
    --from-env-file=/dev/stdin \
    --dry-run=client \
    -o yaml | kubectl apply -f - >/dev/null
  if [[ -n "$(kubectl -n "${NS_OPERATOR}" get secret "${DEMO_SECRET}" -o jsonpath='{.data.OPENAI_API_KEY}' 2>/dev/null || true)" ]]; then
    kubectl -n "${NS_OPERATOR}" patch secret "${DEMO_SECRET}" --type=json \
      -p='[{"op":"remove","path":"/data/OPENAI_API_KEY"}]' >/dev/null
  fi
  unset existing_master_key existing_master_key_with_sentinel master_key
  ok "Runtime Secret ${DEMO_SECRET} created without printing key values"
}

load_local_operator_image() {
  local operator_image="${OPERATOR_IMAGE}:${OPERATOR_IMAGE_TAG}"
  if command -v docker >/dev/null 2>&1 && docker image inspect "${operator_image}" >/dev/null 2>&1; then
    kind load docker-image "${operator_image}" --name "${CLUSTER_NAME}" >/dev/null
    ok "Loaded local operator image ${operator_image}"
  else
    log "Local operator image not found. Helm will pull ${operator_image}."
  fi
}

wait_for_demo_components() {
  step "Waiting for webhook, operator, and LiteLLM"
  kubectl -n "${NS_OPERATOR}" wait \
    --for=condition=Ready \
    "certificate/${DEMO_RELEASE}-agentic-operator-webhook-cert" \
    --timeout="${HELM_TIMEOUT}" >/dev/null

  local operator_deployment
  operator_deployment="$(kubectl -n "${NS_OPERATOR}" get deployment \
    -l app.kubernetes.io/name=agentic-operator \
    -o jsonpath='{.items[0].metadata.name}')"
  [[ -n "${operator_deployment}" ]] || die "operator Deployment not found"
  kubectl -n "${NS_OPERATOR}" rollout status "deployment/${operator_deployment}" --timeout="${HELM_TIMEOUT}" >/dev/null
  kubectl -n "${NS_OPERATOR}" rollout status "deployment/${DEMO_RELEASE}-litellm" --timeout="${HELM_TIMEOUT}" >/dev/null

  local deadline endpoint_ip
  deadline=$(( $(date +%s) + 120 ))
  while (( $(date +%s) < deadline )); do
    endpoint_ip="$(kubectl -n "${NS_OPERATOR}" get endpoints "${DEMO_RELEASE}-webhook-service" \
      -o jsonpath='{.subsets[0].addresses[0].ip}' 2>/dev/null || true)"
    [[ -n "${endpoint_ip}" ]] && break
    sleep 2
  done
  [[ -n "${endpoint_ip:-}" ]] || die "webhook Service has no ready endpoint"
  ok "Webhook, operator, and LiteLLM are ready"
}

prepare_real_demo() {
  step "Preparing real-provider booth demo"
  require_real_provider_keys
  require_command kubectl
  require_command helm
  ensure_demo_kind_cluster
  install_cert_manager
  ensure_namespace "${NS_OPERATOR}"
  create_runtime_provider_secret

  step "Applying CRDs before the Helm release"
  kubectl apply -f "${REPO_ROOT}/config/crd/bases" >/dev/null
  kubectl wait --for=condition=Established crd/agentworkloads.agentic.clawdlinux.org --timeout=60s >/dev/null
  ok "AgentWorkload CRD established"

  load_local_operator_image

  step "Installing Clawdlinux booth components"
  helm upgrade --install "${DEMO_RELEASE}" "${REPO_ROOT}/charts" \
    --namespace "${NS_OPERATOR}" \
    --create-namespace \
    --set-string license.key=dev-license \
    --set argo.enabled=false \
    --set browserless.enabled=false \
    --set minio.enabled=false \
    --set postgresql.enabled=false \
    --set clawdlinuxObservability.enabled=false \
    --set networkPolicy.enabled=true \
    --set ciliumPolicy.enabled=false \
    --set global.runtimeSandbox.enabled=true \
    --set global.runtimeSandbox.createRuntimeClass=true \
    --set agenticOperator.webhook.enabled=true \
    --set-string agentic-operator.env.ENABLE_WEBHOOKS=true \
    --set-string agentic-operator.env.AGENTIC_COST_TRACKING=memory \
    --set agentic-operator.image.repository="${OPERATOR_IMAGE}" \
    --set agentic-operator.image.tag="${OPERATOR_IMAGE_TAG}" \
    --set agentic-operator.image.pullPolicy=IfNotPresent \
    --set litellm.enabled=true \
    --set litellm.builtinOpenAIModelsEnabled=false \
    --set litellm.replicaCount=1 \
    --set-string litellm.resources.requests.memory=1Gi \
    --set-string litellm.resources.limits.memory=2Gi \
    --set-string litellm.existingSecret="${DEMO_SECRET}" \
    --timeout "${HELM_TIMEOUT}" \
    --wait >/dev/null

  kubectl apply -f - >/dev/null <<YAML
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: ${DEMO_RELEASE}-litellm-provider-egress
  namespace: ${NS_OPERATOR}
  labels:
    app.kubernetes.io/component: networkpolicy
    app.kubernetes.io/part-of: clawdlinux-booth-demo
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/name: litellm
  policyTypes:
    - Egress
  egress:
    - to:
        - ipBlock:
            cidr: 0.0.0.0/0
      ports:
        - port: 443
          protocol: TCP
YAML

  kubectl -n "${NS_OPERATOR}" rollout restart deployment/"${DEMO_RELEASE}"-litellm >/dev/null

  wait_for_demo_components
  ok "Preparation complete. Run: $(basename "$0") --present"
}

assert_runtime_secret_shape() {
  local secret_keys required_key expected_keys
  secret_keys="$(kubectl -n "${NS_OPERATOR}" get secret "${DEMO_SECRET}" \
    -o go-template='{{range $key, $value := .data}}{{$key}}{{"\n"}}{{end}}' | LC_ALL=C sort)"
  for required_key in ANTHROPIC_API_KEY LITELLM_MASTER_KEY api-key; do
    grep -Fxq "${required_key}" <<<"${secret_keys}" || die "Secret ${DEMO_SECRET} is missing key ${required_key}. Run --prepare again."
  done
  expected_keys=$'ANTHROPIC_API_KEY\nLITELLM_MASTER_KEY\napi-key'
  [[ "${secret_keys}" == "${expected_keys}" ]] || die "Secret ${DEMO_SECRET} has unexpected key names. Run --prepare again."
  ok "Runtime Secret has the exact showcase key allowlist"
}

ANF_TEMP_FILE=""
ANF_OUTPUT_TEMP_FILE=""
ANF_SANITIZED_TEMP_FILE=""
WORKLOAD_TEMP_FILE=""
WORKLOAD_SOURCE_TEMP_FILE=""
AUDIT_TEMP_FILE=""

cleanup_showcase_temp_files() {
  [[ -z "${ANF_TEMP_FILE}" ]] || rm -f "${ANF_TEMP_FILE}"
  [[ -z "${ANF_OUTPUT_TEMP_FILE}" ]] || rm -f "${ANF_OUTPUT_TEMP_FILE}"
  [[ -z "${ANF_SANITIZED_TEMP_FILE}" ]] || rm -f "${ANF_SANITIZED_TEMP_FILE}"
  [[ -z "${WORKLOAD_TEMP_FILE}" ]] || rm -f "${WORKLOAD_TEMP_FILE}"
  [[ -z "${WORKLOAD_SOURCE_TEMP_FILE}" ]] || rm -f "${WORKLOAD_SOURCE_TEMP_FILE}"
  [[ -z "${AUDIT_TEMP_FILE}" ]] || rm -f "${AUDIT_TEMP_FILE}"
  ANF_TEMP_FILE=""
  ANF_OUTPUT_TEMP_FILE=""
  ANF_SANITIZED_TEMP_FILE=""
  WORKLOAD_TEMP_FILE=""
  WORKLOAD_SOURCE_TEMP_FILE=""
  AUDIT_TEMP_FILE=""
}

cleanup_port_forward() {
  if [[ -n "${PORT_FORWARD_PID}" ]]; then
    kill "${PORT_FORWARD_PID}" >/dev/null 2>&1 || true
    wait "${PORT_FORWARD_PID}" 2>/dev/null || true
    PORT_FORWARD_PID=""
  fi
}

cleanup_showcase_resources() {
  cleanup_port_forward
  cleanup_showcase_temp_files
}

find_anf_snapshot() {
  if [[ ! -x "${REPO_ROOT}/bin/anf-snapshot" ]]; then
    make -C "${REPO_ROOT}" build-anf-snapshot >/dev/null
  fi
  [[ -x "${REPO_ROOT}/bin/anf-snapshot" ]] || die "failed to build bin/anf-snapshot"
  printf '%s' "${REPO_ROOT}/bin/anf-snapshot"
}

capture_anf_context() {
  step "LIVE: Kubernetes state translated to Agent Native Format"
  local snapshot size summary
  snapshot="$(find_anf_snapshot)"
  ANF_TEMP_FILE="$(mktemp "${TMPDIR:-/tmp}/clawdlinux-anf.XXXXXX")"
  chmod 600 "${ANF_TEMP_FILE}"
  ANF_OUTPUT_TEMP_FILE="$(mktemp "${TMPDIR:-/tmp}/clawdlinux-anf-output.XXXXXX")"
  chmod 600 "${ANF_OUTPUT_TEMP_FILE}"
  "${snapshot}" --namespace "${NS_OPERATOR}" --output "${ANF_TEMP_FILE}" >"${ANF_OUTPUT_TEMP_FILE}"
  summary="$(grep -m1 '^ANF context:' "${ANF_OUTPUT_TEMP_FILE}" || true)"
  [[ -n "${summary}" ]] || die "ANF snapshot summary is missing"
  printf '%s\n' "${summary}"
  size="$(wc -c <"${ANF_TEMP_FILE}" | tr -d '[:space:]')"
  [[ "${size}" =~ ^[0-9]+$ ]] || die "could not measure ANF context"
  ((size > 0)) || die "ANF context is empty"
  ((size <= 32768)) || die "ANF context exceeds 32 KiB demo limit"
}

sanitize_anf_context() {
  local source_path="$1"
  local output_path="$2"
  python3 - "${source_path}" "${output_path}" <<'PY'
import os
import sys
import unicodedata

source_path, output_path = sys.argv[1:]
try:
    with open(source_path, "rb") as source:
        raw = source.read()

    text = raw.decode("utf-8")
    if any(
      unicodedata.category(char) in ("Zl", "Zp")
      or (unicodedata.category(char) == "Cc" and char not in "\t\n")
      for char in text
    ):
        raise ValueError
    reserved = ("BEGIN ANF CONTEXT", "END ANF CONTEXT", "ANF_CONTEXT_INSERT_HERE")
    if any(token in text for token in reserved):
        raise ValueError

    lines = text.split("\n")
    if any(line.startswith("?") for line in lines):
        raise ValueError

    sanitized = "\n".join(f"ANF_DATA {line}" for line in lines)
except (OSError, UnicodeError, ValueError):
    raise SystemExit("ANF context rejected by safety policy")

fd = os.open(output_path, os.O_WRONLY | os.O_TRUNC, 0o600)
with os.fdopen(fd, "w", encoding="utf-8") as output:
    output.write(sanitized)
PY
}

build_research_workload_json() {
  WORKLOAD_TEMP_FILE="$(mktemp "${TMPDIR:-/tmp}/clawdlinux-agentworkload.XXXXXX.json")"
  chmod 600 "${WORKLOAD_TEMP_FILE}"
  WORKLOAD_SOURCE_TEMP_FILE="$(mktemp "${TMPDIR:-/tmp}/clawdlinux-agentworkload-source.XXXXXX.json")"
  chmod 600 "${WORKLOAD_SOURCE_TEMP_FILE}"
  ANF_SANITIZED_TEMP_FILE="$(mktemp "${TMPDIR:-/tmp}/clawdlinux-anf-sanitized.XXXXXX")"
  chmod 600 "${ANF_SANITIZED_TEMP_FILE}"
  sanitize_anf_context "${ANF_TEMP_FILE}" "${ANF_SANITIZED_TEMP_FILE}"
  kubectl apply --dry-run=client -f "${RESEARCH_MANIFEST}" -o json >"${WORKLOAD_SOURCE_TEMP_FILE}"
  python3 - "${WORKLOAD_SOURCE_TEMP_FILE}" "${ANF_SANITIZED_TEMP_FILE}" "${WORKLOAD_TEMP_FILE}" <<'PY'
import json
import os
import sys

source_path, anf_path, output_path = sys.argv[1:]
with open(source_path, encoding="utf-8") as source:
    manifest = json.load(source)
with open(anf_path, encoding="utf-8") as anf_source:
    anf = anf_source.read()

objective = manifest["spec"]["objective"]
marker = "ANF_CONTEXT_INSERT_HERE"
if objective.count(marker) != 1:
    raise SystemExit("AgentWorkload objective must contain exactly one ANF marker")
manifest["spec"]["objective"] = objective.replace(marker, anf)

fd = os.open(output_path, os.O_WRONLY | os.O_TRUNC, 0o600)
with os.fdopen(fd, "w", encoding="utf-8") as output:
    json.dump(manifest, output, separators=(",", ":"))
    output.write("\n")
PY
  rm -f "${WORKLOAD_SOURCE_TEMP_FILE}"
  WORKLOAD_SOURCE_TEMP_FILE=""
}

apply_research_workload() {
  step "LIVE: Claude-routed AgentWorkload through in-cluster LiteLLM"
  kubectl -n "${NS_OPERATOR}" delete agentworkload booth-incident-investigation \
    --ignore-not-found --wait=true --timeout=30s >/dev/null

  capture_anf_context
  narration_pause
  build_research_workload_json

  local agentctl_command=""
  local agentctl_source=""
  if [[ -x "${REPO_ROOT}/bin/agentctl" ]]; then
    agentctl_command="${REPO_ROOT}/bin/agentctl"
    agentctl_source="repo-local agentctl"
  elif command -v agentctl >/dev/null 2>&1; then
    agentctl_command="$(command -v agentctl)"
    agentctl_source="agentctl from PATH"
  fi

  if [[ -n "${agentctl_command}" ]]; then
    if "${agentctl_command}" apply -f "${WORKLOAD_TEMP_FILE}" >/dev/null; then
      ok "Applied with ${agentctl_source}"
      cleanup_showcase_temp_files
      return
    fi
    warn "${agentctl_source} failed. Falling back to kubectl."
  fi

  kubectl apply -f "${WORKLOAD_TEMP_FILE}" >/dev/null
  if [[ -n "${agentctl_command}" ]]; then
    ok "agentctl failed. Applied with kubectl."
  else
    ok "agentctl unavailable. Applied with kubectl."
  fi
  cleanup_showcase_temp_files
}

wait_for_research_completion() {
  local deadline phase
  deadline=$(( $(date +%s) + 240 ))
  while (( $(date +%s) < deadline )); do
    phase="$(kubectl -n "${NS_OPERATOR}" get agentworkload booth-incident-investigation \
      -o jsonpath='{.status.phase}' 2>/dev/null || true)"
    case "${phase}" in
      Completed)
        ok "AgentWorkload reached Completed"
        return
        ;;
      Failed|PolicyDenied)
        die "AgentWorkload reached terminal phase ${phase}"
        ;;
    esac
    sleep 2
  done
  die "AgentWorkload did not reach Completed. Last phase: ${phase:-empty}"
}

assert_nonzero_routing_tokens() {
  local routing_message="$1"
  if [[ ! "${routing_message}" =~ input:([0-9]+)[[:space:]]tokens,[[:space:]]output:([0-9]+)[[:space:]]tokens ]] ||
    ((10#${BASH_REMATCH[1]} <= 0 || 10#${BASH_REMATCH[2]} <= 0)); then
    die "routing condition has missing or zero token counts"
  fi
}

assert_claude_routing() {
  local routing_message="$1"
  [[ " ${routing_message} " == *" litellm/clawdlinux-anthropic "* ]] ||
    die "routing condition does not prove litellm/clawdlinux-anthropic"
  assert_nonzero_routing_tokens "${routing_message}"
}

show_real_routing_and_cost() {
  step "LIVE: Claude routing, token, and cost evidence"
  local routing_message cost_annotation metric_output metric_line metric_value
  routing_message="$(kubectl -n "${NS_OPERATOR}" get agentworkload booth-incident-investigation \
    -o jsonpath='{range .status.conditions[?(@.type=="ModelRoutingSucceeded")]}{.message}{end}')"
  [[ -n "${routing_message}" ]] || die "ModelRoutingSucceeded condition is missing"
  assert_claude_routing "${routing_message}"
  printf 'Model routing: %s\n' "${routing_message}"

  cost_annotation="$(kubectl -n "${NS_OPERATOR}" get agentworkload booth-incident-investigation \
    -o go-template='{{index .metadata.annotations "agentworkload.clawdlinux.io/cost-usd-today"}}')"
  awk -v cost="${cost_annotation:-0}" 'BEGIN { exit !(cost + 0 > 0) }' || die "cost annotation is missing or zero"
  printf 'Cost annotation: $%s\n' "${cost_annotation}"

  local pod_name local_port
  pod_name="$(operator_pod_name)"
  [[ -n "${pod_name}" ]] || die "operator pod not found"
  local_port="${DEMO_METRICS_PORT:-18080}"
  kubectl -n "${NS_OPERATOR}" port-forward "pod/${pod_name}" "${local_port}:8080" >/dev/null 2>&1 &
  PORT_FORWARD_PID=$!
  if [[ -n "${PORT_FORWARD_PID_FILE:-}" ]]; then
    printf '%s' "${PORT_FORWARD_PID}" >"${PORT_FORWARD_PID_FILE}"
  fi
  metric_output=""
  for _ in {1..20}; do
    metric_output="$(curl -fsS --max-time 2 "http://127.0.0.1:${local_port}/metrics" 2>/dev/null || true)"
    [[ -n "${metric_output}" ]] && break
    sleep 1
  done
  cleanup_port_forward

  metric_line="$(printf '%s\n' "${metric_output}" |
    grep '^clawdlinux_agent_cost_dollars{' |
    grep -E '[{,]workload="booth-incident-investigation"([,}])' |
    grep -E '[{,]namespace="'"${NS_OPERATOR}"'"([,}])' |
    grep -E '[{,]model="litellm/clawdlinux-anthropic"([,}])' |
    head -1 || true)"
  [[ -n "${metric_line}" ]] || die "Claude cost metric is missing for the booth workload"
  metric_value="$(awk '{print $NF}' <<<"${metric_line}")"
  awk -v cost="${metric_value:-0}" 'BEGIN { exit !(cost + 0 > 0) }' || die "clawdlinux_agent_cost_dollars is zero"
  printf 'Cost metric: %s\n' "${metric_line}"
  narration_pause
}

show_gvisor_configuration_proof() {
  step "SIMULATION / CONFIGURATION PROOF: gVisor admission mutation"
  local runtime_class
  runtime_class="$(kubectl -n "${NS_OPERATOR}" apply --dry-run=server -o jsonpath='{.spec.runtimeClassName}' -f - <<'YAML'
apiVersion: v1
kind: Pod
metadata:
  name: booth-gvisor-dry-run
  labels:
    agentic.clawdlinux.org/runtime-sandbox: gvisor
spec:
  restartPolicy: Never
  containers:
    - name: pause
      image: registry.k8s.io/pause:3.10
YAML
)"
  [[ "${runtime_class}" == "gvisor" ]] || die "webhook dry-run did not inject runtimeClassName=gvisor"
  printf 'SIMULATION / CONFIGURATION PROOF: server-side dry-run injected runtimeClassName=%s. No pod was scheduled.\n' "${runtime_class}"
}

show_network_policy_presence() {
  step "NetworkPolicy object"
  local policy_names
  policy_names="$(kubectl -n "${NS_OPERATOR}" get networkpolicy -o name)"
  [[ -n "${policy_names}" ]] || die "NetworkPolicy object not found"
  printf '%s\n' "${policy_names}"
  printf 'NETWORKPOLICY OBJECT PRESENCE ONLY. Packet enforcement requires an enforcing CNI.\n'
  narration_pause
}

find_audit_verifier() {
  if [[ -x "${REPO_ROOT}/bin/audit-verify" ]]; then
    printf '%s' "${REPO_ROOT}/bin/audit-verify"
    return
  fi
  if command -v audit-verify >/dev/null 2>&1; then
    command -v audit-verify
    return
  fi
  require_command go
  mkdir -p "${REPO_ROOT}/bin"
  go build -o "${REPO_ROOT}/bin/audit-verify" "${REPO_ROOT}/cmd/audit-verify"
  printf '%s' "${REPO_ROOT}/bin/audit-verify"
}

run_prior_run_audit() {
  local tamper="${1:-false}"
  step "PRIOR-RUN HMAC-SIGNED AUDIT FIXTURE: offline verification"
  [[ -f "${AUDIT_FIXTURE}" ]] || die "missing prior-run audit fixture: ${AUDIT_FIXTURE}"
  printf 'PRIOR-RUN ARTIFACT: the current AgentWorkload did not generate this file.\n'

  local verifier
  verifier="$(find_audit_verifier)"
  "${verifier}" --source jsonl --path "${AUDIT_FIXTURE}" --key "${AUDIT_DEMO_KEY}"
  narration_pause

  if [[ "${tamper}" != "true" ]]; then
    return
  fi

  AUDIT_TEMP_FILE="$(mktemp "${TMPDIR:-/tmp}/clawdlinux-audit-tampered.XXXXXX.jsonl")"
  chmod 600 "${AUDIT_TEMP_FILE}"
  sed 's/"actor":"policy-analyst"/"actor":"tampered-actor"/' "${AUDIT_FIXTURE}" > "${AUDIT_TEMP_FILE}"
  if cmp -s "${AUDIT_FIXTURE}" "${AUDIT_TEMP_FILE}"; then
    rm -f "${AUDIT_TEMP_FILE}"
    AUDIT_TEMP_FILE=""
    die "tamper operation did not change the prior-run fixture"
  fi
  if "${verifier}" --source jsonl --path "${AUDIT_TEMP_FILE}" --key "${AUDIT_DEMO_KEY}"; then
    rm -f "${AUDIT_TEMP_FILE}"
    AUDIT_TEMP_FILE=""
    die "tampered audit artifact unexpectedly verified"
  fi
  rm -f "${AUDIT_TEMP_FILE}"
  AUDIT_TEMP_FILE=""
  ok "Tampered prior-run artifact was rejected"
}

print_present_summary() {
  cat <<'EOF'

CURRENT --present EVIDENCE
- Live Kubernetes state translated into ANF context for the AgentWorkload objective.
- Claude completion with genuine input/output tokens and nonzero cost evidence.
- Webhook mutation simulation/configuration proof for runtimeClassName=gvisor. No pod was scheduled.
- NetworkPolicy object presence only. Packet enforcement was not tested.
- Prior-run HMAC-signed audit fixture verification. Optional tamper failure.
EOF
}

present_real_demo() {
  require_command kubectl
  require_command curl
  require_command python3
  require_demo_context
  [[ -f "${RESEARCH_MANIFEST}" ]] || die "missing ${RESEARCH_MANIFEST}"
  assert_runtime_secret_shape
  trap cleanup_showcase_resources EXIT
  trap 'cleanup_showcase_resources; exit 129' HUP
  trap 'cleanup_showcase_resources; exit 130' INT
  trap 'cleanup_showcase_resources; exit 143' TERM
  apply_research_workload
  wait_for_research_completion
  show_real_routing_and_cost
  show_gvisor_configuration_proof
  show_network_policy_presence
  run_prior_run_audit "${TAMPER_AUDIT}"
  print_present_summary
  cleanup_showcase_resources
  trap - EXIT HUP INT TERM
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

install_platform_profile() {
  step "Installing platform profile with Argo and shared services"
  HELM_TIMEOUT="${HELM_TIMEOUT}" bash "${REPO_ROOT}/tests/harness/setup.sh" || {
    die "setup.sh failed. Rerun with --profile lean to omit Argo and shared services."
  }
  wait_for_operator
}

install_lean_profile() {
  step "Installing lean profile without Argo or shared services"
  local operator_image="${OPERATOR_IMAGE}:${OPERATOR_IMAGE_TAG}"
  if command -v kind >/dev/null 2>&1 && docker image inspect "${operator_image}" >/dev/null 2>&1; then
    kind load docker-image "${operator_image}" --name "${CLUSTER_NAME}" >/dev/null
    ok "Loaded local operator image: ${operator_image}"
  fi
  ensure_namespace "${NS_OPERATOR}"
  kubectl apply -f "${REPO_ROOT}/config/crd/agentworkload_crd.yaml" >/dev/null

  # Suppress webhook resources and process registration in the lean profile.
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
    --set agentic-operator.env.ENABLE_WEBHOOKS=false \
    --set agentic-operator.image.repository="${OPERATOR_IMAGE}" \
    --set agentic-operator.image.tag="${OPERATOR_IMAGE_TAG}" \
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
  kubectl -n "${NS_OPERATOR}" get agentworkload \
    -o custom-columns='NAME:.metadata.name,PHASE:.status.phase,OBJECTIVE:.spec.objective' || true

  local action confidence
  action="$(kubectl -n "${NS_OPERATOR}" get agentworkload opa-allow-demo \
    -o jsonpath='{.status.executedActions[-1].name}' 2>/dev/null || true)"
  confidence="$(kubectl -n "${NS_OPERATOR}" get agentworkload opa-allow-demo \
    -o jsonpath='{.status.executedActions[-1].confidence}' 2>/dev/null || true)"
  if [[ -n "${action}" ]]; then
    printf '%bExecuted action:%b %s (confidence %s)\n' "${GREEN}" "${RESET}" "${action}" "${confidence:-unknown}"
  fi
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
  metric_line="$(printf '%s\n' "${metric_output}" | grep 'clawdlinux_agent_cost_dollars' | grep -v '^#' | head -1 || true)"
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

  warn "Cost tracking: clawdlinux_agent_cost_dollars not available"
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
  step "LEGACY DEFAULT FLOW: Applying OPA allow workload"
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

  step "LEGACY DEFAULT FLOW: Applying OPA deny workload"
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
  if [[ "${WITH_SWARM}" != "true" ]]; then
    SWARM_PHASE="skipped (use --with-swarm)"
    return
  fi

  step "Optional scenario: Multi-Agent Swarm"
  if [[ "${DEMO_PROFILE}" == "lean" ]]; then
    warn "Swarm skipped because the lean profile does not install Argo"
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
  printf '%bLegacy Default Flow Summary%b\n' "${BOLD}" "${RESET}"
  printf 'ALLOW path: %s. DENY path: %s. gVisor configuration: %s. NetworkPolicy object: %s. Cost: %s. Swarm: %s.\n' \
    "${ALLOW_PHASE:-<empty>}" \
    "${DENY_PHASE:-<empty>}" \
    "${GVISOR_OK:-no}" \
    "${NETWORK_POLICY_OK:-no}" \
    "${COST_OK:-no}" \
    "${SWARM_PHASE:-skipped (use --with-swarm)}"
  echo ""
  if [[ "${DENY_PHASE:-}" != "PolicyDenied" ]]; then
    warn "Legacy OPA DENY needs a reachable HTTPS MCP endpoint. Set DEMO_MCP_ENDPOINT for development runs."
  fi
}

main() {
  parse_args "$@"
  start_recording_if_requested

  if [[ "${CLEANUP}" == "true" ]]; then
    cleanup_cluster
    return
  fi

  case "${DEMO_MODE}" in
    prepare)
      prepare_real_demo
      return
      ;;
    present)
      present_real_demo
      return
      ;;
    legacy)
      if [[ "${TAMPER_AUDIT}" == "true" ]]; then
        run_prior_run_audit true
        return
      fi
      ;;
  esac

  case "${DEMO_PROFILE}" in
    platform|lean) ;;
    *)
      die "invalid profile: ${DEMO_PROFILE} (expected platform or lean)"
      ;;
  esac
  if [[ "${WITH_SWARM}" == "true" && "${DEMO_PROFILE}" == "lean" ]]; then
    die "--with-swarm requires --profile platform because the swarm uses Argo"
  fi
  check_prerequisites
  ensure_cluster
  if [[ "${DEMO_PROFILE}" == "lean" ]]; then
    install_lean_profile
  else
    install_platform_profile
  fi
  verify_gvisor
  verify_network_policy
  run_opa_demo
  run_swarm_demo
  print_summary
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  main "$@"
fi