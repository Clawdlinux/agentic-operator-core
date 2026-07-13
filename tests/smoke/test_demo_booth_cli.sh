#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
DEMO_SCRIPT="${ROOT_DIR}/scripts/demo-booth.sh"
RECORD_SCRIPT="${ROOT_DIR}/scripts/record-demo.sh"
RESEARCH_MANIFEST="${ROOT_DIR}/examples/research-agent.yaml"
SMOKE_RUNNER="${ROOT_DIR}/tests/smoke/run_smoke.sh"
TEST_TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/demo-booth-cli.XXXXXX")"
trap 'rm -rf "${TEST_TMP_DIR}"' EXIT

bash -n "${DEMO_SCRIPT}"
bash -n "${RECORD_SCRIPT}"
help_output="$("${DEMO_SCRIPT}" --help)"

for flag in --prepare --present --tamper-audit; do
  if ! grep -Fq -- "${flag}" <<<"${help_output}"; then
    printf 'missing booth mode in --help: %s\n' "${flag}" >&2
    exit 1
  fi
done

present_summary="$(bash -c 'source "$1"; print_present_summary' _ "${DEMO_SCRIPT}")"
present_evidence=(
  'CURRENT --present EVIDENCE'
  '- Real OpenAI-routed model call through LiteLLM.'
  '- Genuine input/output tokens plus nonzero cost metric and annotation.'
  '- Separate Anthropic reachability check.'
  '- Webhook mutation simulation/configuration proof for runtimeClassName=gvisor. No pod was scheduled.'
  '- NetworkPolicy object presence only. Packet enforcement was not tested.'
  '- Prior-run HMAC-signed audit fixture verification. Optional tamper failure.'
)
for evidence_line in "${present_evidence[@]}"; do
  if ! grep -Fxq -- "${evidence_line}" <<<"${present_summary}"; then
    printf 'present summary is missing evidence boundary: %s\n' "${evidence_line}" >&2
    exit 1
  fi
done
if grep -Eiq 'OPA|policy (decision|gate)|current-run (signed|audit|attestation)' <<<"${present_summary}"; then
  printf 'present summary must not claim current-run policy or signed-audit evidence\n' >&2
  exit 1
fi

if ! bash -c 'source "$1"; assert_nonzero_routing_tokens "Task classified as research, routed to openai/gpt-4o-mini (input:21 tokens, output:8 tokens)"' _ "${DEMO_SCRIPT}"; then
  printf 'present path must accept genuine nonzero routing token counts\n' >&2
  exit 1
fi
set +e
zero_token_output="$(bash -c 'source "$1"; assert_nonzero_routing_tokens "Task classified as research, routed to openai/gpt-4o-mini (input:0 tokens, output:8 tokens)"' _ "${DEMO_SCRIPT}" 2>&1)"
zero_token_status=$?
set -e
if [[ ${zero_token_status} -eq 0 ]] || ! grep -Fq 'routing condition has missing or zero token counts' <<<"${zero_token_output}"; then
  printf 'present path must reject missing or zero routing token counts\n' >&2
  exit 1
fi

required_text=(
  'SIMULATION / CONFIGURATION PROOF'
  'NETWORKPOLICY OBJECT PRESENCE ONLY. Packet enforcement requires an enforcing CNI.'
  'PRIOR-RUN ARTIFACT'
  'OPENAI_API_KEY'
  'ANTHROPIC_API_KEY'
  'clawdlinux-demo-litellm'
)

for text in "${required_text[@]}"; do
  if ! grep -Fq -- "${text}" "${DEMO_SCRIPT}"; then
    printf 'missing booth safety or credential marker: %s\n' "${text}" >&2
    exit 1
  fi
done

if grep -Eq '^[[:space:]]*set[[:space:]]+-[^[:space:]]*x' "${DEMO_SCRIPT}"; then
  printf 'demo script must not enable shell tracing\n' >&2
  exit 1
fi

set +e
prepare_output="$(env -u OPENAI_API_KEY -u ANTHROPIC_API_KEY DEMO_ENV_FILE="${TEST_TMP_DIR}/missing.env" PATH=/usr/bin:/bin "${DEMO_SCRIPT}" --prepare 2>&1)"
prepare_status=$?
set -e
if [[ ${prepare_status} -eq 0 ]]; then
  printf 'prepare must fail when provider keys are absent\n' >&2
  exit 1
fi
if ! grep -Fq 'Real-provider preparation requires: OPENAI_API_KEY ANTHROPIC_API_KEY' <<<"${prepare_output}"; then
  printf 'prepare did not fail at the provider key gate\n' >&2
  exit 1
fi

env_file="${TEST_TMP_DIR}/credentials.env"
fake_bin="${TEST_TMP_DIR}/fake-bin"
command_log="${TEST_TMP_DIR}/commands.log"
captured_master_key_file="${TEST_TMP_DIR}/master-key.txt"
network_policy_file="${TEST_TMP_DIR}/provider-egress.yaml"
mkdir -p "${fake_bin}"
file_openai='demo-file-openai-not-secret'
environment_openai='demo-environment-openai-not-secret'
file_anthropic='demo-file-anthropic-not-secret'
cat >"${env_file}" <<EOF
export OPENAI_API_KEY='${file_openai}'
ANTHROPIC_API_KEY="${file_anthropic}"
IGNORED_VALUE=not-loaded
EOF

cat >"${fake_bin}/kind" <<'EOF'
#!/usr/bin/env bash
printf 'kind' >>"${FAKE_COMMAND_LOG}"
printf ' %q' "$@" >>"${FAKE_COMMAND_LOG}"
printf '\n' >>"${FAKE_COMMAND_LOG}"
if [[ "${1:-}" == "get" && "${2:-}" == "clusters" ]]; then
  printf '%s\n' "${CLUSTER_NAME:-clawdlinux-demo}"
fi
EOF

cat >"${fake_bin}/helm" <<'EOF'
#!/usr/bin/env bash
printf 'helm' >>"${FAKE_COMMAND_LOG}"
printf ' %q' "$@" >>"${FAKE_COMMAND_LOG}"
printf '\n' >>"${FAKE_COMMAND_LOG}"
EOF

cat >"${fake_bin}/openssl" <<'EOF'
#!/usr/bin/env bash
printf 'openssl' >>"${FAKE_COMMAND_LOG}"
printf ' %q' "$@" >>"${FAKE_COMMAND_LOG}"
printf '\n' >>"${FAKE_COMMAND_LOG}"
printf 'generated-test-master-key\n'
EOF

cat >"${fake_bin}/base64" <<'EOF'
#!/usr/bin/env bash
if [[ "${1:-}" == "--decode" ]]; then
  exit 64
fi
if [[ "${1:-}" == "-D" ]]; then
  exec /usr/bin/base64 --decode
fi
exec /usr/bin/base64 "$@"
EOF

cat >"${fake_bin}/kubectl" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf 'kubectl' >>"${FAKE_COMMAND_LOG}"
printf ' %q' "$@" >>"${FAKE_COMMAND_LOG}"
printf '\n' >>"${FAKE_COMMAND_LOG}"
args=" $* "
case "${args}" in
  *" config current-context "*)
    printf 'kind-%s\n' "${CLUSTER_NAME:-clawdlinux-demo}"
    ;;
  *" get deployment "*)
    printf 'agentic-operator\n'
    ;;
  *" get endpoints "*)
    printf '127.0.0.1\n'
    ;;
  *" get secret "*"LITELLM_MASTER_KEY"*)
    if [[ -n "${MOCK_EXISTING_MASTER_KEY:-}" ]]; then
      printf '%s' "${MOCK_EXISTING_MASTER_KEY}" | base64
    fi
    ;;
  *" create namespace "*)
    printf 'apiVersion: v1\nkind: Namespace\nmetadata:\n  name: test\n'
    ;;
  *" create secret generic "*)
    secret_input="$(cat)"
    grep -Fqx "OPENAI_API_KEY=${EXPECTED_OPENAI_API_KEY}" <<<"${secret_input}"
    grep -Fqx "ANTHROPIC_API_KEY=${EXPECTED_ANTHROPIC_API_KEY}" <<<"${secret_input}"
    master_key="$(sed -n 's/^LITELLM_MASTER_KEY=//p' <<<"${secret_input}")"
    api_key="$(sed -n 's/^api-key=//p' <<<"${secret_input}")"
    [[ "${master_key}" == sk-* && ${#master_key} -ge 19 && "${master_key}" == "${api_key}" ]]
    if [[ -n "${CAPTURED_MASTER_KEY_FILE:-}" ]]; then
      printf '%s' "${master_key}" >"${CAPTURED_MASTER_KEY_FILE}"
    fi
    printf 'apiVersion: v1\nkind: Secret\nmetadata:\n  name: test\n'
    ;;
  *" apply -f - "*)
    apply_input="$(cat)"
    if grep -Fqx 'kind: NetworkPolicy' <<<"${apply_input}" && [[ -n "${NETWORK_POLICY_FILE:-}" ]]; then
      printf '%s\n' "${apply_input}" >"${NETWORK_POLICY_FILE}"
    fi
    ;;
esac
EOF
chmod +x "${fake_bin}/kind" "${fake_bin}/helm" "${fake_bin}/openssl" "${fake_bin}/base64" "${fake_bin}/kubectl"

set +e
loaded_prepare_output="$(env \
  -u ANTHROPIC_API_KEY \
  -u LITELLM_MASTER_KEY \
  OPENAI_API_KEY="${environment_openai}" \
  DEMO_ENV_FILE="${env_file}" \
  FAKE_COMMAND_LOG="${command_log}" \
  CAPTURED_MASTER_KEY_FILE="${captured_master_key_file}" \
  NETWORK_POLICY_FILE="${network_policy_file}" \
  EXPECTED_OPENAI_API_KEY="${environment_openai}" \
  EXPECTED_ANTHROPIC_API_KEY="${file_anthropic}" \
  PATH="${fake_bin}:/usr/bin:/bin" \
  "${DEMO_SCRIPT}" --prepare 2>&1)"
loaded_prepare_status=$?
set -e
if [[ ${loaded_prepare_status} -ne 0 ]]; then
  printf 'prepare with a safe env file failed:\n%s\n' "${loaded_prepare_output}" >&2
  exit 1
fi
for status_line in 'OPENAI_API_KEY=available' 'ANTHROPIC_API_KEY=available'; do
  if ! grep -Fxq "${status_line}" <<<"${loaded_prepare_output}"; then
    printf 'prepare did not report credential availability: %s\n' "${status_line}" >&2
    exit 1
  fi
done
generated_master_key="$(<"${captured_master_key_file}")"
if [[ "${generated_master_key}" != 'sk-generated-test-master-key' ]]; then
  printf 'prepare did not generate the expected sk-prefixed master key\n' >&2
  exit 1
fi
if grep -Fq "${generated_master_key}" <<<"${loaded_prepare_output}" || grep -Fq "${generated_master_key}" "${command_log}"; then
  printf 'generated master key leaked into prepare output or command log\n' >&2
  exit 1
fi
for helm_arg in \
  '--set-string litellm.resources.requests.memory=1Gi' \
  '--set-string litellm.resources.limits.memory=2Gi'; do
  if ! grep -Fq -- "${helm_arg}" "${command_log}"; then
    printf 'prepare did not pass LiteLLM memory argument: %s\n' "${helm_arg}" >&2
    exit 1
  fi
done
network_policy_contracts=(
  'kind: NetworkPolicy'
  '  name: clawdlinux-demo-litellm-provider-egress'
  '      app.kubernetes.io/name: litellm'
  '            cidr: 0.0.0.0/0'
  '        - port: 443'
  '          protocol: TCP'
)
for contract in "${network_policy_contracts[@]}"; do
  if ! grep -Fxq -- "${contract}" "${network_policy_file}"; then
    printf 'provider egress policy is missing contract: %s\n' "${contract}" >&2
    exit 1
  fi
done

reused_master_key='sk-existing-master-key-123456'
: >"${command_log}"
reuse_output="$(env \
  -u LITELLM_MASTER_KEY \
  OPENAI_API_KEY="${environment_openai}" \
  ANTHROPIC_API_KEY="${file_anthropic}" \
  DEMO_ENV_FILE="${TEST_TMP_DIR}/missing.env" \
  MOCK_EXISTING_MASTER_KEY="${reused_master_key}" \
  CAPTURED_MASTER_KEY_FILE="${captured_master_key_file}" \
  NETWORK_POLICY_FILE="${network_policy_file}" \
  FAKE_COMMAND_LOG="${command_log}" \
  EXPECTED_OPENAI_API_KEY="${environment_openai}" \
  EXPECTED_ANTHROPIC_API_KEY="${file_anthropic}" \
  PATH="${fake_bin}:/usr/bin:/bin" \
  "${DEMO_SCRIPT}" --prepare 2>&1)"
if [[ "$(<"${captured_master_key_file}")" != "${reused_master_key}" ]]; then
  printf 'prepare did not preserve a valid existing master key\n' >&2
  exit 1
fi
if grep -Fq "${reused_master_key}" <<<"${reuse_output}" || grep -Fq "${reused_master_key}" "${command_log}"; then
  printf 'reused master key leaked into prepare output or command log\n' >&2
  exit 1
fi

malformed_master_keys=($'sk-existing-master-key\r' $'sk-existing-master-key\n')
for malformed_master_key in "${malformed_master_keys[@]}"; do
  : >"${command_log}"
  env \
    -u LITELLM_MASTER_KEY \
    OPENAI_API_KEY="${environment_openai}" \
    ANTHROPIC_API_KEY="${file_anthropic}" \
    DEMO_ENV_FILE="${TEST_TMP_DIR}/missing.env" \
    MOCK_EXISTING_MASTER_KEY="${malformed_master_key}" \
    CAPTURED_MASTER_KEY_FILE="${captured_master_key_file}" \
    NETWORK_POLICY_FILE="${network_policy_file}" \
    FAKE_COMMAND_LOG="${command_log}" \
    EXPECTED_OPENAI_API_KEY="${environment_openai}" \
    EXPECTED_ANTHROPIC_API_KEY="${file_anthropic}" \
    PATH="${fake_bin}:/usr/bin:/bin" \
    "${DEMO_SCRIPT}" --prepare >/dev/null 2>&1
  if [[ "$(<"${captured_master_key_file}")" != 'sk-generated-test-master-key' ]]; then
    printf 'prepare must replace malformed existing master keys\n' >&2
    exit 1
  fi
done
if ! grep -Fq "Credential variable OPENAI_API_KEY loaded from environment" <<<"${loaded_prepare_output}"; then
  printf 'prepare did not report environment precedence for OPENAI_API_KEY\n' >&2
  exit 1
fi
if ! grep -Fq "Credential variable ANTHROPIC_API_KEY loaded from ${env_file}" <<<"${loaded_prepare_output}"; then
  printf 'prepare did not report the env file source for ANTHROPIC_API_KEY\n' >&2
  exit 1
fi
for dummy_value in "${file_openai}" "${environment_openai}" "${file_anthropic}"; do
  if grep -Fq "${dummy_value}" <<<"${loaded_prepare_output}" || grep -Fq "${dummy_value}" "${command_log}"; then
    printf 'credential value leaked into prepare output or command log\n' >&2
    exit 1
  fi
done

duplicate_env_file="${TEST_TMP_DIR}/duplicate.env"
cat >"${duplicate_env_file}" <<'EOF'
OPENAI_API_KEY=first-placeholder
export OPENAI_API_KEY='second-placeholder'
ANTHROPIC_API_KEY=anthropic-placeholder
EOF
set +e
duplicate_output="$(env -u OPENAI_API_KEY -u ANTHROPIC_API_KEY DEMO_ENV_FILE="${duplicate_env_file}" PATH=/usr/bin:/bin "${DEMO_SCRIPT}" --prepare 2>&1)"
duplicate_status=$?
set -e
if [[ ${duplicate_status} -eq 0 ]] || ! grep -Fq 'duplicate definition for OPENAI_API_KEY' <<<"${duplicate_output}"; then
  printf 'prepare must reject duplicate credential definitions\n' >&2
  exit 1
fi

multiline_env_file="${TEST_TMP_DIR}/multiline.env"
cat >"${multiline_env_file}" <<'EOF'
OPENAI_API_KEY="unsafe
newline"
ANTHROPIC_API_KEY=anthropic-placeholder
EOF
set +e
multiline_output="$(env -u OPENAI_API_KEY -u ANTHROPIC_API_KEY DEMO_ENV_FILE="${multiline_env_file}" PATH=/usr/bin:/bin "${DEMO_SCRIPT}" --prepare 2>&1)"
multiline_status=$?
set -e
if [[ ${multiline_status} -eq 0 ]] || ! grep -Fq 'malformed definition for OPENAI_API_KEY' <<<"${multiline_output}"; then
  printf 'prepare must reject multiline credential values\n' >&2
  exit 1
fi

set +e
environment_newline_output="$(env \
  OPENAI_API_KEY=$'line-one\nline-two' \
  ANTHROPIC_API_KEY=anthropic-placeholder \
  DEMO_ENV_FILE="${TEST_TMP_DIR}/missing.env" \
  PATH=/usr/bin:/bin \
  "${DEMO_SCRIPT}" --prepare 2>&1)"
environment_newline_status=$?
set -e
if [[ ${environment_newline_status} -eq 0 ]] || ! grep -Fq 'unsafe newline in OPENAI_API_KEY from environment' <<<"${environment_newline_output}"; then
  printf 'prepare must reject newlines in exported credential values\n' >&2
  exit 1
fi

unsafe_master_values=($'line-one\nline-two' $'line-one\rline-two')
for unsafe_master_value in "${unsafe_master_values[@]}"; do
  : >"${command_log}"
  set +e
  master_key_output="$(env \
    OPENAI_API_KEY=openai-placeholder \
    ANTHROPIC_API_KEY=anthropic-placeholder \
    LITELLM_MASTER_KEY="${unsafe_master_value}" \
    DEMO_ENV_FILE="${TEST_TMP_DIR}/missing.env" \
    FAKE_COMMAND_LOG="${command_log}" \
    PATH="${fake_bin}:/usr/bin:/bin" \
    "${DEMO_SCRIPT}" --prepare 2>&1)"
  master_key_status=$?
  set -e
  if [[ ${master_key_status} -eq 0 ]] || ! grep -Fq 'unsafe newline in LITELLM_MASTER_KEY from environment' <<<"${master_key_output}"; then
    printf 'prepare must reject LF and CR in exported LITELLM_MASTER_KEY\n' >&2
    exit 1
  fi
  if grep -Fq 'create secret generic' "${command_log}"; then
    printf 'prepare must validate LITELLM_MASTER_KEY before Secret creation\n' >&2
    exit 1
  fi
done

short_master_key='sk-too-short'
: >"${command_log}"
set +e
short_master_output="$(env \
  OPENAI_API_KEY=openai-placeholder \
  ANTHROPIC_API_KEY=anthropic-placeholder \
  LITELLM_MASTER_KEY="${short_master_key}" \
  DEMO_ENV_FILE="${TEST_TMP_DIR}/missing.env" \
  FAKE_COMMAND_LOG="${command_log}" \
  PATH="${fake_bin}:/usr/bin:/bin" \
  "${DEMO_SCRIPT}" --prepare 2>&1)"
short_master_status=$?
set -e
if [[ ${short_master_status} -eq 0 ]] || ! grep -Fq 'contain at least 16 key characters' <<<"${short_master_output}"; then
  printf 'prepare must reject short LITELLM master keys\n' >&2
  exit 1
fi
if grep -Fq 'create secret generic' "${command_log}"; then
  printf 'prepare must reject short master keys before Secret creation\n' >&2
  exit 1
fi

fallback_root="${TEST_TMP_DIR}/fallback-root"
fallback_bin="${TEST_TMP_DIR}/fallback-bin"
fallback_log="${TEST_TMP_DIR}/fallback.log"
mkdir -p "${fallback_root}/examples" "${fallback_bin}"
: >"${fallback_root}/examples/research-agent.yaml"
cat >"${fallback_bin}/agentctl" <<'EOF'
#!/usr/bin/env bash
printf 'agentctl' >>"${FAKE_COMMAND_LOG}"
printf ' %q' "$@" >>"${FAKE_COMMAND_LOG}"
printf '\n' >>"${FAKE_COMMAND_LOG}"
exit 23
EOF
cat >"${fallback_bin}/kubectl" <<'EOF'
#!/usr/bin/env bash
printf 'kubectl' >>"${FAKE_COMMAND_LOG}"
printf ' %q' "$@" >>"${FAKE_COMMAND_LOG}"
printf '\n' >>"${FAKE_COMMAND_LOG}"
EOF
chmod +x "${fallback_bin}/agentctl" "${fallback_bin}/kubectl"

set +e
fallback_output="$(FAKE_COMMAND_LOG="${fallback_log}" PATH="${fallback_bin}:/usr/bin:/bin" bash -c '
  source "$1"
  REPO_ROOT="$2"
  RESEARCH_MANIFEST="$2/examples/research-agent.yaml"
  NS_OPERATOR="test-routing"
  apply_research_workload
' _ "${DEMO_SCRIPT}" "${fallback_root}" 2>&1)"
fallback_status=$?
set -e
if [[ ${fallback_status} -ne 0 ]]; then
  printf 'failed agentctl did not fall back to kubectl:\n%s\n' "${fallback_output}" >&2
  exit 1
fi
if ! grep -Fq 'agentctl apply -f' "${fallback_log}" || ! grep -Fq 'kubectl apply -f' "${fallback_log}"; then
  printf 'fallback path did not attempt agentctl then kubectl\n' >&2
  exit 1
fi

if ! grep -Fq 'test_demo_booth_cli.sh' "${SMOKE_RUNNER}"; then
  printf 'normal smoke runner must include test_demo_booth_cli.sh\n' >&2
  exit 1
fi

if ! grep -Fq -- '--present' "${RECORD_SCRIPT}"; then
  printf 'record-demo.sh must record the present path\n' >&2
  exit 1
fi

set +e
present_output="$(KUBECONFIG=/dev/null "${DEMO_SCRIPT}" --present 2>&1)"
present_status=$?
set -e
if [[ ${present_status} -eq 0 ]] || ! grep -Fq 'wrong kubectl context' <<<"${present_output}"; then
  printf 'present must refuse a non-demo kubectl context\n' >&2
  exit 1
fi

if grep -Eq '^[[:space:]]*orchestration:' "${RESEARCH_MANIFEST}"; then
  printf 'research workload must not declare orchestration\n' >&2
  exit 1
fi
if [[ "$(grep -Fc 'litellm/clawdlinux-openai' "${RESEARCH_MANIFEST}")" -ne 3 ]]; then
  printf 'all 3 task categories must use the LiteLLM OpenAI alias\n' >&2
  exit 1
fi

printf 'demo booth CLI flags: PASS\n'