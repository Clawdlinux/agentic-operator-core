#!/usr/bin/env bash
# demo-claims.sh: prove the decision-architecture claims on a fresh kind cluster.
#
# No API keys. No external model provider. A deterministic mock MCP server
# (scripts/demo/mockmcp) plays the agent. Each scenario prints its heading,
# the exact command, and a PASS or FAIL line. The script exits nonzero if any
# proof fails.
#
# Build note: go.mod carries a dev-only replace for receiptspec
# (../agentgate-receiptspec, see BLOCKERS.md), so the repo Dockerfile cannot
# build. This script builds linux binaries on the host and packages them with
# scripts/demo/Dockerfile.demo, then loads that image into kind. That detour
# goes away once agentgate v0.1.4 is tagged and the replace is dropped.
#
# Prerequisites: docker (running), kind, kubectl, helm, go, python3, curl,
# openssl. Optional: ../agentgate-receiptspec for the AgentGate verifier.
#
# Usage: scripts/demo-claims.sh [--keep] [--skip-build] [--cluster-name NAME]
#                               [--dry-run] [--help]
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
CHECK="${SCRIPT_DIR}/demo/claims_check.py"

CLUSTER_NAME="clawdlinux-claims"
KEEP=false
SKIP_BUILD=false
DRY_RUN=false
NS="agentic-system"
DEMO_NS="claims-demo"
RELEASE="claims"
IMAGE="clawdlinux-claims-demo:dev"
BUILD_DIR="${DEMO_BUILD_DIR:-/tmp/demo-claims-build}"
CERT_MANAGER_VERSION="${CERT_MANAGER_VERSION:-v1.17.2}"
HELM_TIMEOUT="${HELM_TIMEOUT:-240s}"
PF_PORT="${PF_PORT:-18080}"
MOCK_HOST="mock-mcp.${DEMO_NS}.svc"
MOCK_URL="https://${MOCK_HOST}"
RECEIPTSPEC_DIR="${RECEIPTSPEC_DIR:-${REPO_ROOT}/../agentgate-receiptspec}"
MODEL_JSON="${REPO_ROOT}/tools/decision-train/out/model.json"
SYNTH_APPROVALS="${REPO_ROOT}/tools/decision-train/out/approvals.jsonl"
FAKE_AWS_KEY="AKIAIOSFODNN7EXAMPLE"
TEST_AADHAAR="234123412346"

usage() {
  cat <<EOF
Usage: scripts/demo-claims.sh [flags]

Proves the decision-architecture claims on a fresh kind cluster with a mock
MCP server. No API keys. Exits nonzero if any proof fails.

Flags:
  --keep               keep the kind cluster and demo image at the end
  --skip-build         reuse binaries and image from ${BUILD_DIR}
  --cluster-name NAME  kind cluster name (default ${CLUSTER_NAME})
  --dry-run            print the plan and exit
  -h, --help           show this help
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --keep) KEEP=true ;;
    --skip-build) SKIP_BUILD=true ;;
    --cluster-name) [[ $# -ge 2 ]] || { usage >&2; exit 2; }; CLUSTER_NAME="$2"; shift ;;
    --dry-run) DRY_RUN=true ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown flag: $1" >&2; usage >&2; exit 2 ;;
  esac
  shift
done

BIN="${BUILD_DIR}/bin"
PF_PID=""

say() { printf '%s\n' "$*"; }
heading() { printf '\n==== %s ====\n' "$*"; }
note() { printf '  %s\n' "$*"; }
# run prints the exact command, then runs it.
run() { printf '$ %s\n' "$*"; "$@"; }
die() { printf 'ERROR: %s\n' "$*" >&2; exit 1; }
check() {
  local desc="$1"; shift
  if "$@"; then note "ok: ${desc}"; else note "NOT OK: ${desc}"; return 1; fi
}

kc() { kubectl --context "kind-${CLUSTER_NAME}" "$@"; }
agentctl() { "${BIN}/agentctl" --kubeconfig "${KUBECONFIG_FILE}" "$@"; }

plan() {
  cat <<EOF
Plan (cluster ${CLUSTER_NAME}):
  0. build linux binaries on the host, package ${IMAGE}, kind load, install
     cert-manager ${CERT_MANAGER_VERSION} and the chart with receipts required,
     the AgentWorkload webhook on, and the mock MCP server at ${MOCK_URL}
  1. clean action to a declared destination: Completed, receipt before execute
  2. claimed confidence 0.99 and health 100, Aadhaar to undeclared destination:
     invariant deny, PolicyDenied, receipt, execute_action never called
  3. fake AKIA key in params: INV-01 deny
  4. dpdp-in@v0.1.0, decisionType automated, personal data, permissive mode:
     PendingApproval
  5. forged approval-by rejected by the webhook, agentctl approve stamped with
     the real identity (HMAC stamp key Secret), human receipt carries the
     identity digest, approvals.jsonl example; a restored consumed approval
     does not run again; a human edit that adds a credential is denied by INV-01
  6. receipts export and verify under the pin made at install, manifest
     removal and a wrong pin rejected, tamper checks, AgentGate verifier
     agreement, dataset verify
  7. decision model in shadow via the chart value, receipt model block,
     agentctl decision eval WARNING
  8. a credential past the 64 KiB scan window: INV-06 deny
  9. operator restarted with a wrong writer pin: INV-05 deny, nothing runs
 10. trace coverage via make trace-coverage (no collector)
 11. cleanup (kind delete) unless --keep
EOF
}

if [[ "${DRY_RUN}" == true ]]; then
  plan
  exit 0
fi

WORK="$(mktemp -d /tmp/demo-claims.XXXXXX)"
RESULTS="${WORK}/results"
TOKEN_FILE="${WORK}/token"
# A private kubeconfig, so the demo never edits ~/.kube/config.
KUBECONFIG_FILE="${WORK}/kubeconfig"
: >"${RESULTS}"

cleanup() {
  local rc=$?
  [[ -n "${PF_PID}" ]] && kill "${PF_PID}" >/dev/null 2>&1 || true
  if [[ "${KEEP}" == true ]]; then
    say "Keeping cluster ${CLUSTER_NAME} and image ${IMAGE}. Work dir: ${WORK}"
  else
    kind delete cluster --name "${CLUSTER_NAME}" --kubeconfig "${KUBECONFIG_FILE}" >/dev/null 2>&1 || true
    docker image rm "${IMAGE}" >/dev/null 2>&1 || true
    say "Deleted kind cluster ${CLUSTER_NAME} and image ${IMAGE}. Work dir kept: ${WORK}"
  fi
  exit "${rc}"
}
trap cleanup EXIT

for c in docker kind kubectl helm go python3 curl openssl; do
  command -v "$c" >/dev/null 2>&1 || die "missing prerequisite: $c"
done
docker info >/dev/null 2>&1 || die "docker daemon is not running"

# ---------------------------------------------------------------- build ----
build() {
  heading "Build (host binaries, demo image)"
  local arch
  arch="$(go env GOARCH)"
  mkdir -p "${BIN}" "${BUILD_DIR}/ctx"
  (
    cd "${REPO_ROOT}"
    run go build -o "${BIN}/agentctl" ./cmd/agentctl
    run env GOOS=linux GOARCH="${arch}" CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o "${BUILD_DIR}/ctx/manager" ./cmd
    run env GOOS=linux GOARCH="${arch}" CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o "${BUILD_DIR}/ctx/receipt-writer" ./cmd/receipt-writer
    run env GOOS=linux GOARCH="${arch}" CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o "${BUILD_DIR}/ctx/mockmcp" ./scripts/demo/mockmcp
  )
  rm -f "${BIN}/agentgate-verify"
  if [[ -d "${RECEIPTSPEC_DIR}/cmd/agentgate-verify" ]]; then
    (cd "${RECEIPTSPEC_DIR}" && run go build -o "${BIN}/agentgate-verify" ./cmd/agentgate-verify)
  else
    note "AgentGate verifier source not found at ${RECEIPTSPEC_DIR}. Cross-check will be skipped."
  fi

  # Throwaway CA and serving cert for the mock MCP server. Demo only.
  local ctx="${BUILD_DIR}/ctx"
  openssl req -x509 -newkey rsa:2048 -nodes -days 2 -subj "/CN=claims-demo-ca" \
    -keyout "${BUILD_DIR}/ca.key" -out "${ctx}/ca.crt" >/dev/null 2>&1
  openssl req -newkey rsa:2048 -nodes -subj "/CN=${MOCK_HOST}" \
    -keyout "${ctx}/tls.key" -out "${BUILD_DIR}/tls.csr" >/dev/null 2>&1
  printf 'subjectAltName=DNS:%s,DNS:%s.cluster.local\nextendedKeyUsage=serverAuth\n' "${MOCK_HOST}" "${MOCK_HOST}" >"${BUILD_DIR}/san.ext"
  openssl x509 -req -in "${BUILD_DIR}/tls.csr" -CA "${ctx}/ca.crt" -CAkey "${BUILD_DIR}/ca.key" \
    -CAcreateserial -days 2 -extfile "${BUILD_DIR}/san.ext" -out "${ctx}/tls.crt" >/dev/null 2>&1
  run docker build -q -f "${SCRIPT_DIR}/demo/Dockerfile.demo" -t "${IMAGE}" "${ctx}"
}

if [[ "${SKIP_BUILD}" == true ]]; then
  heading "Build skipped (--skip-build)"
  [[ -x "${BIN}/agentctl" ]] || die "no agentctl in ${BIN}; run without --skip-build"
  docker image inspect "${IMAGE}" >/dev/null 2>&1 || die "image ${IMAGE} missing; run without --skip-build"
else
  build
fi

# --------------------------------------------------------------- cluster ---
heading "Cluster ${CLUSTER_NAME}"
if kind get clusters 2>/dev/null | grep -Fxq "${CLUSTER_NAME}"; then
  note "deleting existing cluster ${CLUSTER_NAME} for a fresh start"
  kind delete cluster --name "${CLUSTER_NAME}" >/dev/null
fi
run kind create cluster --name "${CLUSTER_NAME}" --kubeconfig "${KUBECONFIG_FILE}" --wait 120s
export KUBECONFIG="${KUBECONFIG_FILE}"
run kind load docker-image "${IMAGE}" --name "${CLUSTER_NAME}"

heading "Install cert-manager ${CERT_MANAGER_VERSION}"
run helm --kube-context "kind-${CLUSTER_NAME}" upgrade --install cert-manager oci://quay.io/jetstack/charts/cert-manager \
  --namespace cert-manager --create-namespace --version "${CERT_MANAGER_VERSION}" \
  --set crds.enabled=true --timeout "${HELM_TIMEOUT}" --wait >/dev/null

heading "Install Clawdlinux (receipts required, AgentWorkload webhook on)"
kc create namespace "${NS}" >/dev/null
kc create namespace "${DEMO_NS}" >/dev/null
kc apply -f "${REPO_ROOT}/config/crd/bases" >/dev/null
kc wait --for=condition=Established crd/agentworkloads.agentic.clawdlinux.org --timeout=60s >/dev/null
key_file="${WORK}/signing-key.hex"
(umask 077 && openssl rand -hex 32 >"${key_file}")
kc -n "${NS}" create secret generic receipt-signing-key \
  --from-file=signing-key="${key_file}" >/dev/null
# Pin the writer public key so the operator verifies every receipt it gets.
"${BIN}/agentctl" receipts trust-root --signing-key-file "${key_file}" >"${WORK}/writer-trust.json"
rm -f "${key_file}"
kc -n "${NS}" create configmap receipt-trust-root \
  --from-file=trust.json="${WORK}/writer-trust.json" >/dev/null
note "signing key Secret and pinned trust root ConfigMap created (key not printed, key file removed)"
stamp_file="${WORK}/approval-stamp.key"
(umask 077 && openssl rand -hex 32 >"${stamp_file}")
kc -n "${NS}" create secret generic approval-stamp-key \
  --from-file=stamp.key="${stamp_file}" >/dev/null
rm -f "${stamp_file}"
note "approval stamp HMAC key Secret created (key not printed, key file removed)"
image_repo="${IMAGE%%:*}"
image_tag="${IMAGE##*:}"
# kindnet enforces NetworkPolicy. The chart's default-deny egress has no rule
# for an MCP endpoint, so allow the kind pod CIDR on the mock's port only.
run helm --kube-context "kind-${CLUSTER_NAME}" upgrade --install "${RELEASE}" "${REPO_ROOT}/charts" --namespace "${NS}" \
  --set 'networkPolicy.additionalAllowedHosts[0].cidr=10.244.0.0/16' \
  --set 'networkPolicy.additionalAllowedHosts[0].ports[0].port=8443' \
  --set 'networkPolicy.additionalAllowedHosts[0].ports[0].protocol=TCP' \
  --set-string license.key=dev-license \
  --set argo.enabled=false --set browserless.enabled=false --set litellm.enabled=false \
  --set minio.enabled=false --set postgresql.enabled=false \
  --set clawdlinuxObservability.enabled=false --set ciliumPolicy.enabled=false \
  --set webUI.enabled=false \
  --set global.runtimeSandbox.enabled=false \
  --set agenticOperator.webhook.enabled=true \
  --set agenticOperator.webhook.agentWorkloadEnabled=true \
  --set-string agentic-operator.env.ENABLE_WEBHOOKS=true \
  --set-string agentic-operator.env.SSL_CERT_FILE=/demo/ca.crt \
  --set agentic-operator.leaderElection=false \
  --set agentic-operator.image.repository="${image_repo}" \
  --set agentic-operator.image.tag="${image_tag}" \
  --set agentic-operator.image.pullPolicy=IfNotPresent \
  --set global.receipts.enabled=true \
  --set global.receipts.required=true \
  --set global.receipts.signingKey.existingSecret=receipt-signing-key \
  --set global.receipts.trustRoot.existingConfigMap=receipt-trust-root \
  --set global.approvals.stampKey.existingSecret=approval-stamp-key \
  --set global.receipts.image.repository="${image_repo}" \
  --set global.receipts.image.tag="${image_tag}" \
  --set global.receipts.image.pullPolicy=IfNotPresent \
  --timeout "${HELM_TIMEOUT}" --wait >/dev/null
OPERATOR_SEL="app.kubernetes.io/name=agentic-operator,app.kubernetes.io/component=operator"
kc -n "${NS}" wait --for=condition=Ready "certificate/${RELEASE}-agentic-operator-webhook-cert" --timeout="${HELM_TIMEOUT}" >/dev/null
kc -n "${NS}" rollout status statefulset/"${RELEASE}-receipt-writer" --timeout="${HELM_TIMEOUT}" >/dev/null
kc -n "${NS}" wait --for=condition=Ready pod -l "${OPERATOR_SEL}" --timeout="${HELM_TIMEOUT}" >/dev/null
kc -n "${NS}" get secret "${RELEASE}-receipt-writer-token" -o jsonpath='{.data.token}' | base64 -d >"${TOKEN_FILE}"
chmod 600 "${TOKEN_FILE}"

heading "Mock MCP server at ${MOCK_URL}"
kc apply -f - >/dev/null <<YAML
apiVersion: apps/v1
kind: Deployment
metadata: {name: mock-mcp, namespace: ${DEMO_NS}}
spec:
  replicas: 1
  selector: {matchLabels: {app: mock-mcp}}
  template:
    metadata: {labels: {app: mock-mcp}}
    spec:
      securityContext: {runAsNonRoot: true, runAsUser: 65532, seccompProfile: {type: RuntimeDefault}}
      containers:
        - name: mock-mcp
          image: ${IMAGE}
          imagePullPolicy: IfNotPresent
          command: ["/mockmcp"]
          ports: [{containerPort: 8443}]
          readinessProbe: {httpGet: {path: /healthz, port: 8443, scheme: HTTPS}}
          securityContext: {allowPrivilegeEscalation: false, readOnlyRootFilesystem: true, capabilities: {drop: ["ALL"]}}
---
apiVersion: v1
kind: Service
metadata: {name: mock-mcp, namespace: ${DEMO_NS}}
spec:
  selector: {app: mock-mcp}
  ports: [{port: 443, targetPort: 8443}]
YAML
kc -n "${DEMO_NS}" rollout status deployment/mock-mcp --timeout=120s >/dev/null
note "mock MCP ready (TLS, CA baked into the demo image, operator trusts it via SSL_CERT_FILE)"

# --------------------------------------------------------------- helpers ---
start_port_forward() {
  [[ -n "${PF_PID}" ]] && kill "${PF_PID}" >/dev/null 2>&1 || true
  kc -n "${NS}" port-forward "svc/${RELEASE}-receipt-writer" "${PF_PORT}:8080" >"${WORK}/port-forward.log" 2>&1 &
  PF_PID=$!
  local i
  for i in $(seq 1 30); do
    curl -fsS "http://127.0.0.1:${PF_PORT}/healthz" >/dev/null 2>&1 && return 0
    sleep 1
  done
  die "receipt-writer port-forward did not come up"
}
start_port_forward
WRITER="http://127.0.0.1:${PF_PORT}"

# apply_workload NAME SCENARIO MODE [extra spec yaml, indented 2]
apply_workload() {
  local name="$1" scenario="$2" mode="$3" extra="${4:-}"
  local manifest="${WORK}/${name}.yaml"
  cat >"${manifest}" <<YAML
apiVersion: agentic.clawdlinux.org/v1alpha1
kind: AgentWorkload
metadata:
  name: ${name}
  namespace: ${DEMO_NS}
spec:
  objective: "scenario=${scenario}"
  workloadType: generic
  agents: ["demo-agent"]
  opaPolicy: ${mode}
  autoApproveThreshold: "0.95"
  mcpServerEndpoint: "${MOCK_URL}"
${extra}
YAML
  sed 's/^/  | /' "${manifest}"
  printf '$ kubectl apply -f %s\n' "${manifest}"
  local i out
  # The webhook can lag cert-manager CA injection on a fresh cluster.
  for i in $(seq 1 40); do
    if out="$(kc apply -f "${manifest}" 2>&1)"; then say "${out}"; return 0; fi
    case "${out}" in
      *"failed calling webhook"*|*"connection refused"*|*"no endpoints"*|*"x509"*) sleep 3 ;;
      *) say "${out}"; return 1 ;;
    esac
  done
  say "${out}"
  return 1
}

wait_phase() {
  local name="$1" want="$2" timeout="${3:-180}" phase="" end
  end=$(( $(date +%s) + timeout ))
  while (( $(date +%s) < end )); do
    phase="$(kc -n "${DEMO_NS}" get agentworkload "${name}" -o jsonpath='{.status.phase}' 2>/dev/null || true)"
    [[ "${phase}" == "${want}" ]] && { note "phase ${name}: ${phase}"; return 0; }
    sleep 2
  done
  note "phase ${name}: '${phase}', wanted ${want}"
  return 1
}

delete_workload() {
  kc -n "${DEMO_NS}" delete agentworkload "$1" --wait=true --timeout=60s >/dev/null 2>&1 || true
}

mock_log() {
  kc -n "${DEMO_NS}" logs deployment/mock-mcp >"${WORK}/mock.log" 2>/dev/null
  printf '%s' "${WORK}/mock.log"
}

calls() { python3 "${CHECK}" calls "$(mock_log)" "$1" "$2"; }

operator_log_clean() {
  local value="$1" log="${WORK}/operator.log"
  kc -n "${NS}" logs -l "${OPERATOR_SEL}" --tail=-1 >"${log}"
  ! grep -Fq "${value}" "${log}"
}

# settle_operator waits after an operator restart. On kind a new pod's first
# connections to the writer and the mock can stall for about 40s. The operator
# then fails closed with INV-05, which is correct but not what a scenario tests.
settle_operator() {
  local secs="${OPERATOR_SETTLE_SECS:-45}"
  note "waiting ${secs}s for the new operator pod's egress to settle"
  sleep "${secs}"
}

export_receipts() {
  rm -rf "$1"
  run agentctl receipts export --writer "${WRITER}" --token-file "${TOKEN_FILE}" --out "$1" >/dev/null
}

export_dataset() {
  rm -rf "$1"
  run agentctl dataset export --writer "${WRITER}" --token-file "${TOKEN_FILE}" --out "$1" >/dev/null
}

condition_message() {
  kc -n "${DEMO_NS}" get agentworkload "$1" -o jsonpath="{.status.conditions[?(@.type==\"$2\")].message}"
}

eq() { [[ "$1" == "$2" ]]; }
contains() { [[ "$1" == *"$2"* ]]; }

# scenario NUMBER TITLE FUNCTION runs the function in a subshell with errexit
# on, so one failed proof does not stop the others.
scenario() {
  local num="$1" title="$2" fn="$3" rc
  heading "Scenario ${num}: ${title}"
  set +e
  ( set -e; "${fn}" )
  rc=$?
  set -e
  # Remove this scenario's workloads even if a proof failed half way.
  kc -n "${DEMO_NS}" delete agentworkloads --all --wait=true --timeout=90s >/dev/null 2>&1 || true
  if [[ "${rc}" -eq 0 ]]; then
    say "PASS ${num}: ${title}"; printf 'PASS %s: %s\n' "${num}" "${title}" >>"${RESULTS}"
  else
    say "FAIL ${num}: ${title}"; printf 'FAIL %s: %s\n' "${num}" "${title}" >>"${RESULTS}"
  fi
}

DPDP_INTENT="  declaredIntent:
    purpose: \"refund decisions for KYC verified customers\"
    decisionType: automated
    allowedDataClasses: [aadhaar]
    allowedDestinations: [\"${MOCK_HOST}\"]
  policyPacks: [\"dpdp-in@v0.1.0\"]"

# ------------------------------------------------------------- scenarios ---
s1() {
  apply_workload s1-clean s1-clean strict "  declaredIntent:
    purpose: \"scale the web tier\"
    decisionType: assisted
    allowedDataClasses: []
    allowedDestinations: [\"${MOCK_HOST}\"]"
  wait_phase s1-clean Completed
  export_receipts "${WORK}/s1"
  run python3 "${CHECK}" records "${WORK}/s1" s1-clean
  check "exactly one allow receipt stored before the first execute_action" \
    python3 "${CHECK}" write-ahead "${WORK}/s1" s1-clean "$(mock_log)" s1-clean
  delete_workload s1-clean
}

s2() {
  apply_workload s2-aadhaar s2-aadhaar strict "  declaredIntent:
    purpose: \"sync CRM records\"
    decisionType: assisted
    allowedDataClasses: []
    allowedDestinations: [\"crm.partner.example\"]"
  wait_phase s2-aadhaar PolicyDenied
  local msg
  msg="$(condition_message s2-aadhaar PolicyDenied)"
  note "PolicyDenied message: ${msg}"
  check "condition names INV-03 (personal data to undeclared destination)" contains "${msg}" "INV-03"
  export_receipts "${WORK}/s2"
  run python3 "${CHECK}" records "${WORK}/s2" s2-aadhaar
  check "deny receipt from the invariant layer, with the 0.99 and 100 claims recorded" \
    bash -c "python3 '${CHECK}' records '${WORK}/s2' s2-aadhaar | grep -q 'layer=invariant outcome=deny .*INV-03.* claimed_confidence=0.99 claimed_health=100'"
  check "propose_action was called" test "$(calls s2-aadhaar propose_action)" -ge 1
  check "execute_action was never called" eq "$(calls s2-aadhaar execute_action)" 0
  check "operator log does not contain the Aadhaar number" operator_log_clean "${TEST_AADHAAR}"
  delete_workload s2-aadhaar
}

s3() {
  apply_workload s3-credential s3-credential strict ""
  wait_phase s3-credential PolicyDenied
  local msg
  msg="$(condition_message s3-credential PolicyDenied)"
  note "PolicyDenied message: ${msg}"
  check "condition names INV-01" contains "${msg}" "INV-01"
  export_receipts "${WORK}/s3"
  run python3 "${CHECK}" records "${WORK}/s3" s3-credential
  check "deny receipt with INV-01" \
    bash -c "python3 '${CHECK}' records '${WORK}/s3' s3-credential | grep -q 'outcome=deny .*rules=\[INV-01[],]'"
  check "execute_action was never called" eq "$(calls s3-credential execute_action)" 0
  check "operator log does not contain the key" operator_log_clean "${FAKE_AWS_KEY}"
  delete_workload s3-credential
}

s4() {
  apply_workload s4-dpdp s4-dpdp permissive "${DPDP_INTENT}"
  wait_phase s4-dpdp PendingApproval
  local reason msg
  reason="$(kc -n "${DEMO_NS}" get agentworkload s4-dpdp -o jsonpath='{.status.conditions[?(@.type=="ApprovalRequired")].reason}')"
  msg="$(condition_message s4-dpdp ApprovalRequired)"
  note "ApprovalRequired ${reason}: ${msg}"
  check "pack approval finding holds the action in permissive mode" eq "${reason}" "PolicyPackApproval"
  check "finding is dpdp-in@v0.1.0/DPDP-IN-04" contains "${msg}" "dpdp-in@v0.1.0/DPDP-IN-04"
  export_receipts "${WORK}/s4"
  run python3 "${CHECK}" records "${WORK}/s4" s4-dpdp
  check "require_approval receipt from the rules layer" \
    bash -c "python3 '${CHECK}' records '${WORK}/s4' s4-dpdp | grep -q 'layer=rules outcome=require_approval'"
  check "execute_action was never called" eq "$(calls s4-dpdp execute_action)" 0
  delete_workload s4-dpdp
}

s5() {
  local me stamp out pending msg
  local me_digest
  me="$(kc auth whoami -o jsonpath='{.status.userInfo.username}')"
  me_digest="$(python3 "${CHECK}" digest "${me}")"
  note "kubectl identity: ${me}"
  note "identity digest (sha256 of clawdlinux.org/human-identity/v1, NUL, name): ${me_digest}"

  say "-- 5a: forged stamp, then a real approve"
  apply_workload s5-approve s5-approve permissive "${DPDP_INTENT}"
  wait_phase s5-approve PendingApproval
  pending="$(kc -n "${DEMO_NS}" get agentworkload s5-approve -o jsonpath='{.status.pendingApproval.id}')"
  note "pending id: ${pending}"
  kc -n "${DEMO_NS}" get agentworkload s5-approve -o jsonpath='{.status.pendingApproval}' >"${WORK}/s5-pending.json"
  printf '$ kubectl annotate agentworkload s5-approve clawdlinux.org/approval-decision=approve %s\n' \
    "'clawdlinux.org/approval-by={\"username\":\"mallory\",\"groups_sha256\":\"x\"}'"
  if out="$(kc -n "${DEMO_NS}" annotate agentworkload s5-approve clawdlinux.org/approval-decision=approve \
    'clawdlinux.org/approval-by={"username":"mallory","groups_sha256":"x"}' 2>&1)"; then
    say "${out}"; note "NOT OK: the forged stamp was accepted"; return 1
  fi
  say "${out}"
  check "webhook rejected the client-forged approval-by" contains "$(printf '%s' "${out}" | tr '[:upper:]' '[:lower:]')" "forbidden"
  check "no decision recorded after the forged attempt" \
    eq "$(kc -n "${DEMO_NS}" get agentworkload s5-approve -o jsonpath='{.metadata.annotations.clawdlinux\.org/approval-decision}')" ""

  run agentctl -n "${DEMO_NS}" approve s5-approve --reason "checked the refund"
  wait_phase s5-approve Completed
  stamp="$(kc -n "${DEMO_NS}" get agentworkload s5-approve -o jsonpath='{.metadata.annotations.clawdlinux\.org/approval-by}')"
  note "approval-by: ${stamp}"
  check "webhook stamped the real kubectl identity" contains "${stamp}" "\"username\":\"${me}\""
  check "webhook stamped an HMAC over the decision" \
    test -n "$(kc -n "${DEMO_NS}" get agentworkload s5-approve -o jsonpath='{.metadata.annotations.clawdlinux\.org/approval-mac}')"
  msg="$(condition_message s5-approve ApprovalDecision)"
  note "ApprovalDecision message: ${msg}"
  check "status message names the approver" contains "${msg}" "after human approve by ${me}"
  check "execute_action called once after approval" eq "$(calls s5-approve execute_action)" 1
  export_dataset "${WORK}/s5"
  run python3 "${CHECK}" records "${WORK}/s5" s5-approve
  check "layer human receipt with the approver identity digest as principal" \
    bash -c "python3 '${CHECK}' records '${WORK}/s5' s5-approve | grep -q 'layer=human outcome=approved policy_decision=allow principal=${me_digest} '"
  check "no receipt or record carries the raw username" \
    bash -c "! grep -Fq '\"${me}\"' '${WORK}/s5/receipts.jsonl' '${WORK}/s5/records.jsonl'"
  run python3 "${CHECK}" approvals "${WORK}/s5/approvals.jsonl" s5-approve
  check "one approvals.jsonl example" eq "$(python3 "${CHECK}" approvals "${WORK}/s5/approvals.jsonl" s5-approve | tail -1)" 1
  note "example line:"
  grep -F '"s5-approve"' "${WORK}/s5/approvals.jsonl" | head -1 | sed 's/^/  | /'

  say "-- 5c: a restored, already consumed approval does not run again"
  python3 -c 'import json,sys; print(json.dumps({"status": {"phase": "PendingApproval", "pendingApproval": json.load(open(sys.argv[1]))}}))' \
    "${WORK}/s5-pending.json" >"${WORK}/s5-replay.json"
  note "restore the decided pending action; the stamped approval annotations are still on the object"
  run kubectl --context "kind-${CLUSTER_NAME}" -n "${DEMO_NS}" patch agentworkload s5-approve \
    --subresource=status --type=merge --patch-file "${WORK}/s5-replay.json"
  run kubectl --context "kind-${CLUSTER_NAME}" -n "${DEMO_NS}" annotate agentworkload s5-approve demo.clawdlinux.org/nudge=1
  local end reason=""
  end=$(( $(date +%s) + 90 ))
  while (( $(date +%s) < end )); do
    reason="$(kc -n "${DEMO_NS}" get agentworkload s5-approve -o jsonpath='{.status.conditions[?(@.type=="ApprovalDecision")].reason}')"
    [[ "${reason}" == "ApprovalReplay" ]] && break
    sleep 2
  done
  note "ApprovalDecision ${reason}: $(condition_message s5-approve ApprovalDecision)"
  check "controller refused the consumed pending id" eq "${reason}" "ApprovalReplay"
  sleep 5
  check "execute_action still called exactly once" eq "$(calls s5-approve execute_action)" 1
  delete_workload s5-approve

  say "-- 5b: a human edit cannot bypass an invariant"
  apply_workload s5-edit s5-edit strict "${DPDP_INTENT}"
  wait_phase s5-edit PendingApproval
  cat >"${WORK}/edit.json" <<JSON
{"name": "decide_kyc_refund", "description": "refund and upload a backup", "params": {"scenario": "s5-edit", "aws_access_key_id": "${FAKE_AWS_KEY}"}}
JSON
  sed 's/^/  | /' "${WORK}/edit.json"
  run agentctl -n "${DEMO_NS}" edit-approve s5-edit --edit-file "${WORK}/edit.json" --reason "use backup creds"
  wait_phase s5-edit PolicyDenied
  msg="$(condition_message s5-edit PolicyDenied)"
  note "PolicyDenied message: ${msg}"
  check "edited action denied by INV-01" contains "${msg}" "INV-01"
  check "execute_action was never called" eq "$(calls s5-edit execute_action)" 0
  export_dataset "${WORK}/s5b"
  run python3 "${CHECK}" records "${WORK}/s5b" s5-edit
  check "human edit receipt, then an INV-01 deny receipt" \
    bash -c "python3 '${CHECK}' records '${WORK}/s5b' s5-edit | grep -q 'layer=human outcome=edited' && python3 '${CHECK}' records '${WORK}/s5b' s5-edit | grep -q 'outcome=deny .*INV-01'"
  check "edit example stored" bash -c "python3 '${CHECK}' approvals '${WORK}/s5b/approvals.jsonl' s5-edit | grep -q 'label=edit'"
  delete_workload s5-edit
}

expect_fail() {
  local desc="$1"; shift
  printf '$ %s\n' "$*"
  if "$@"; then note "NOT OK: ${desc} (passed, should fail)"; return 1; fi
  note "ok: ${desc} (nonzero exit)"
}

s6() {
  local ev="${WORK}/s6" pinned="${WORK}/writer-trust.json" out
  export_dataset "${ev}"
  note "pinned trust root made at install from the signing key: ${pinned}"
  run agentctl receipts verify "${ev}" --trust-root "${pinned}" | tee "${WORK}/s6-verify.txt"
  out="$(cat "${WORK}/s6-verify.txt")"
  check "completeness proven against the signed export manifest" contains "${out}" "completeness: proven"

  cp -R "${ev}" "${WORK}/s6-no-manifest"
  run python3 "${CHECK}" strip-manifest "${WORK}/s6-no-manifest/receipts.jsonl"
  expect_fail "agentctl rejects an export with the manifest removed" \
    "${BIN}/agentctl" receipts verify "${WORK}/s6-no-manifest" --trust-root "${pinned}"
  run agentctl receipts verify "${WORK}/s6-no-manifest" --trust-root "${pinned}" --allow-prefix
  note "ok: --allow-prefix accepts it and warns that completeness is not proven"

  local wrong_key="${WORK}/wrong-key.hex"
  (umask 077 && openssl rand -hex 32 >"${wrong_key}")
  "${BIN}/agentctl" receipts trust-root --signing-key-file "${wrong_key}" >"${WORK}/wrong-trust.json"
  rm -f "${wrong_key}"
  expect_fail "agentctl rejects the export under a wrong pin" \
    "${BIN}/agentctl" receipts verify "${ev}" --trust-root "${WORK}/wrong-trust.json"
  local agentgate=false
  if [[ -x "${BIN}/agentgate-verify" ]]; then
    agentgate=true
    run "${BIN}/agentgate-verify" --source jsonl --path "${ev}/receipts.jsonl" --trust-root "${pinned}"
    note "ok: AgentGate verifier agrees on the same chain (one shared format)"
  else
    note "SKIP: AgentGate verifier not built (no ${RECEIPTSPEC_DIR})"
  fi

  cp -R "${ev}" "${WORK}/s6-bad-chain"
  run python3 "${CHECK}" flip "${WORK}/s6-bad-chain/receipts.jsonl" '"action":"decision:'
  expect_fail "agentctl rejects a one-byte change in the chain" \
    "${BIN}/agentctl" receipts verify "${WORK}/s6-bad-chain" --trust-root "${pinned}"
  if [[ "${agentgate}" == true ]]; then
    expect_fail "AgentGate verifier rejects the same change" \
      "${BIN}/agentgate-verify" --source jsonl --path "${WORK}/s6-bad-chain/receipts.jsonl" --trust-root "${pinned}"
  fi

  cp -R "${ev}" "${WORK}/s6-bad-record"
  run python3 "${CHECK}" flip "${WORK}/s6-bad-record/records.jsonl" '"outcome":"'
  expect_fail "agentctl rejects a one-byte change in a decision record" \
    "${BIN}/agentctl" receipts verify "${WORK}/s6-bad-record" --trust-root "${pinned}"
  note "AgentGate checks receipts only. Record binding is the agentctl check."

  run agentctl dataset verify "${ev}" --trust-root "${pinned}"
}

s7() {
  run kubectl --context "kind-${CLUSTER_NAME}" -n "${NS}" create configmap decision-model --from-file=model.json="${MODEL_JSON}"
  run helm --kube-context "kind-${CLUSTER_NAME}" upgrade "${RELEASE}" "${REPO_ROOT}/charts" -n "${NS}" --reuse-values \
    --set global.decisionModel.existingConfigMap=decision-model --timeout "${HELM_TIMEOUT}" --wait >/dev/null
  local end
  end=$(( $(date +%s) + 120 ))
  until kc -n "${NS}" logs -l "${OPERATOR_SEL}" --tail=-1 2>/dev/null | grep -q "Decision model loaded"; do
    (( $(date +%s) < end )) || { note "operator never logged 'Decision model loaded'"; return 1; }
    sleep 2
  done
  kc -n "${NS}" logs -l "${OPERATOR_SEL}" --tail=-1 | grep "Decision model loaded" | tail -1 | cut -c1-400
  settle_operator
  apply_workload s7-shadow s7-shadow strict "  declaredIntent:
    purpose: \"scale the web tier\"
    decisionType: assisted
    allowedDataClasses: []
    allowedDestinations: [\"${MOCK_HOST}\"]"
  wait_phase s7-shadow Completed
  export_dataset "${WORK}/s7"
  run python3 "${CHECK}" model "${WORK}/s7" s7-shadow "${MODEL_JSON}"
  check "shadow model did not change the outcome; action executed" test "$(calls s7-shadow execute_action)" -ge 1
  delete_workload s7-shadow

  local out
  run agentctl decision eval --dataset "${WORK}/s7" --model "${MODEL_JSON}" \
    --trust-root "${WORK}/writer-trust.json" --baseline | tee "${WORK}/eval-export.txt"
  out="$(cat "${WORK}/eval-export.txt")"
  check "eval on the exported dataset prints a WARNING" contains "${out}" "WARNING"
  run agentctl decision eval --dataset "${SYNTH_APPROVALS}" --model "${MODEL_JSON}" \
    --allow-unverified --baseline >"${WORK}/eval-synthetic.txt"
  sed -n '1,8p' "${WORK}/eval-synthetic.txt"
  out="$(cat "${WORK}/eval-synthetic.txt")"
  check "eval on the shipped training set warns the data is synthetic" contains "${out}" "are synthetic"
}

s9() {
  note "propose_action returns a 70 KiB string with an AKIA key after it."
  apply_workload s9-oversize s9-oversize strict ""
  wait_phase s9-oversize PolicyDenied
  local msg
  msg="$(condition_message s9-oversize PolicyDenied)"
  note "PolicyDenied message: ${msg}"
  check "condition names INV-06 (scan incomplete)" contains "${msg}" "INV-06"
  export_receipts "${WORK}/s9"
  run python3 "${CHECK}" records "${WORK}/s9" s9-oversize
  check "deny receipt with INV-06" \
    bash -c "python3 '${CHECK}' records '${WORK}/s9' s9-oversize | grep -q 'outcome=deny .*INV-06'"
  check "execute_action was never called" eq "$(calls s9-oversize execute_action)" 0
  check "operator log does not contain the key" operator_log_clean "${FAKE_AWS_KEY}"
  delete_workload s9-oversize
}

s10() {
  note "Repin the operator to a throwaway key. The real writer still signs with its own key."
  local wrong_key="${WORK}/wrong-pin.hex" deploy
  (umask 077 && openssl rand -hex 32 >"${wrong_key}")
  "${BIN}/agentctl" receipts trust-root --signing-key-file "${wrong_key}" >"${WORK}/wrong-pin.json"
  rm -f "${wrong_key}"
  kc -n "${NS}" create configmap receipt-trust-root --from-file=trust.json="${WORK}/wrong-pin.json" \
    --dry-run=client -o yaml | kc apply -f - >/dev/null
  deploy="$(kc -n "${NS}" get deployment -l "${OPERATOR_SEL}" -o name)"
  run kubectl --context "kind-${CLUSTER_NAME}" -n "${NS}" rollout restart "${deploy}"
  kc -n "${NS}" rollout status "${deploy}" --timeout="${HELM_TIMEOUT}" >/dev/null
  settle_operator
  apply_workload s10-wrong-pin s10-wrong-pin strict "  declaredIntent:
    purpose: \"scale the web tier\"
    decisionType: assisted
    allowedDataClasses: []
    allowedDestinations: [\"${MOCK_HOST}\"]"
  wait_phase s10-wrong-pin PolicyDenied
  local msg
  msg="$(condition_message s10-wrong-pin PolicyDenied)"
  note "PolicyDenied message: ${msg}"
  check "condition names INV-05 (receipt required)" contains "${msg}" "INV-05"
  check "execute_action was never called" eq "$(calls s10-wrong-pin execute_action)" 0
  kc -n "${NS}" logs -l "${OPERATOR_SEL}" --tail=-1 >"${WORK}/operator-wrong-pin.log"
  grep -F "unpinned key" "${WORK}/operator-wrong-pin.log" | tail -1 | cut -c1-300 || true
  check "operator refused the writer receipt as signed by an unpinned key" \
    grep -Fq "unpinned key" "${WORK}/operator-wrong-pin.log"
  delete_workload s10-wrong-pin
}

s8() {
  note "No collector here. The Go coverage test asserts the controller emits decision spans."
  (cd "${REPO_ROOT}" && run make trace-coverage) | tee "${WORK}/trace.txt" | grep -vE '^=== RUN|^--- PASS|^PASS$|^ok ' || true
  grep -q '^ok ' "${WORK}/trace.txt"
}

T0=$(date +%s)
scenario 1 "clean action to a declared destination, receipt before execute" s1
scenario 2 "confident agent, Aadhaar to an undeclared destination" s2
scenario 3 "credential in params, INV-01" s3
scenario 4 "dpdp-in pack escalation in permissive mode" s4
scenario 5 "human approve with a stamped identity, edit cannot bypass invariants" s5
scenario 6 "offline verification and tamper detection" s6
scenario 7 "decision model in shadow mode" s7
scenario 8 "credential past the scan window, INV-06" s9
scenario 9 "operator pinned to the wrong writer key, INV-05" s10
scenario 10 "decision tracing coverage (Go test, no collector)" s8

heading "Results ($(( $(date +%s) - T0 ))s of scenarios)"
cat "${RESULTS}"
cat <<'EOF'

Proven by this demo, on a kind cluster:
- invariants deny credentials, undeclared egress, and personal data to an
  undeclared host, whatever the agent claims
- a signed receipt is stored before the action runs, and denied actions are
  never executed
- a policy pack approval finding holds an action even in permissive mode
- the webhook stamps the approver under an HMAC and rejects a forged stamp
- receipts carry the approver identity digest, not the username
- a restored, already consumed approval does not run again
- a human edit is re-checked and cannot bypass an invariant
- receipts verify offline under a pin made at install, a removed manifest
  and a wrong pin fail, tamper is detected, and AgentGate verifies the
  same chain
- a credential past the 64 KiB scan window is denied by INV-06
- an operator pinned to the wrong writer key refuses every receipt and
  denies by INV-05
- the decision model scores in shadow without changing the outcome
- decision spans are covered by a Go test

NOT proven by this demo:
- the runtime-adapter path (spec.orchestration): no invariants, packs, or
  receipts there yet
- Argo approval gates
- execution outcome receipts (only the pre-execution decision is signed)
- production validity of the decision model (trained on synthetic DRAFT data)
- packet-level egress enforcement (not asserted here; the demo opens operator
  egress to the mock on the pod CIDR, and INV-02 checks the configured MCP
  endpoint only)
- caller identity observation (observed.callerIdentity is empty)
- escalate mode on a model that fails to load, and agentctl resourceVersion
  conflicts (unit tests only)
EOF

if grep -q '^FAIL' "${RESULTS}"; then
  say "OVERALL: FAIL"
  exit 1
fi
say "OVERALL: PASS"
