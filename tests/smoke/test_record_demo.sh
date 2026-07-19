#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
RECORDER="${REPO_ROOT}/scripts/record-demo.sh"
TEST_DIR="$(mktemp -d)"
trap 'rm -rf "${TEST_DIR}"' EXIT

[[ -x "${RECORDER}" ]] || {
  printf 'recorder is not executable: %s\n' "${RECORDER}" >&2
  exit 1
}

help_output="$("${RECORDER}" --help)"
grep -Fq 'Records the real booth script and live dashboard as a local WebM file.' <<<"${help_output}"
grep -Fq 'generated under demos/local/ and ignored by Git' <<<"${help_output}"
grep -Fq -- '--scenario NAME' <<<"${help_output}"

recording_path="${TEST_DIR}/end-to-end.webm"
dry_run_output="$(
  DEMO_VIZ_PORT=9876 "${RECORDER}" \
    --dry-run \
    --no-open \
    --pace 3 \
    --duration 60 \
    --start-delay 12 \
    --output "${recording_path}"
)"
grep -Fq 'python3 -u scripts/demo-visualizer.py --present --scenario all --tamper-audit --pace 3' <<<"${dry_run_output}"
grep -Fq 'Dashboard: http://127.0.0.1:9876' <<<"${dry_run_output}"
grep -Fq "Recording: ${recording_path}" <<<"${dry_run_output}"
grep -Fq 'Dry run complete. No apps or recording were started.' <<<"${dry_run_output}"
[[ ! -e "${recording_path}" ]]
[[ ! -e "${recording_path%.webm}.log" ]]

for invalid_args in \
  '--duration 19' \
  '--duration 181' \
  '--start-delay 4' \
  '--start-delay 31' \
  '--pace 61' \
  '--scenario invalid' \
  '--output recording.mov'; do
  set +e
  # shellcheck disable=SC2086
  "${RECORDER}" --dry-run ${invalid_args} >"${TEST_DIR}/invalid.out" 2>&1
  exit_code=$?
  set -e
  if [[ ${exit_code} -eq 0 ]]; then
    printf 'recorder accepted invalid arguments: %s\n' "${invalid_args}" >&2
    exit 1
  fi
done

grep -Fq 'record-dashboard.js' "${RECORDER}"
grep -Fq 'umask 077' "${RECORDER}"
grep -Fq 'recordVideo' "${REPO_ROOT}/scripts/record-dashboard.js"
grep -Fq "stage === 'Live run complete.'" "${REPO_ROOT}/scripts/record-dashboard.js"
grep -Fq 'AgentWorkload reached Completed' "${RECORDER}"
grep -Fq 'Tampered prior-run artifact was rejected' "${RECORDER}"

write_valid_log() {
  local target="$1"
  cat >"${target}" <<'EOF'
Scenario selected: id=success expected=Complete fixture=examples/booth-scenarios/success/fixture.yaml workload=examples/booth-scenarios/success/workload.template.yaml
Scenario result: id=success expected=Complete observed=Complete
ANF context: source=kubernetes/kind-clawdlinux-demo scope=namespace:agentic-system source_bytes=100 source_objects=2 projected_objects=1 unprojected_pods=1 omitted_containers=0 omitted_service_ports=0 omitted_named_target_ports=0 document_json_bytes=80 anf_bytes=40 document_json_tokens_est=20 anf_tokens_est=10 reduction=50.0 top_level_entities=1
[OK] AgentWorkload reached Completed
Provider result: gateway=litellm route=litellm/clawdlinux-anthropic provider=claude input_tokens=10 output_tokens=20
Cost evidence: annotation_usd=0.001 metric_usd=0.001 route=litellm/clawdlinux-anthropic
Scenario selected: id=fault-injection expected=Failed fixture=examples/booth-scenarios/fault-injection/fixture.yaml workload=examples/booth-scenarios/fault-injection/workload.template.yaml
Scenario result: id=fault-injection expected=Failed observed=Failed
ANF context: source=kubernetes/kind-clawdlinux-demo scope=namespace:agentic-system source_bytes=101 source_objects=2 projected_objects=1 unprojected_pods=1 omitted_containers=0 omitted_service_ports=0 omitted_named_target_ports=0 document_json_bytes=81 anf_bytes=41 document_json_tokens_est=20 anf_tokens_est=10 reduction=49.4 top_level_entities=1
[OK] AgentWorkload reached Completed
Provider result: gateway=litellm route=litellm/clawdlinux-anthropic provider=claude input_tokens=11 output_tokens=21
Cost evidence: annotation_usd=0.002 metric_usd=0.002 route=litellm/clawdlinux-anthropic
[OK] Tampered prior-run artifact was rejected
CURRENT --present EVIDENCE
Run finished. Dashboard server stays up -- Ctrl+C to stop.
EOF
}

valid_log="${TEST_DIR}/valid.log"
write_valid_log "${valid_log}"
grep -Fq 'Recorded log verification: PASS' <("${RECORDER}" --scenario all --verify-log "${valid_log}")

missing_log="${TEST_DIR}/missing.log"
grep -v 'Scenario result: id=fault-injection' "${valid_log}" >"${missing_log}"
duplicate_log="${TEST_DIR}/duplicate.log"
cp "${valid_log}" "${duplicate_log}"
printf '%s\n' 'Scenario selected: id=success expected=Complete' >>"${duplicate_log}"
out_of_order_log="${TEST_DIR}/out-of-order.log"
awk '
  /Scenario selected: id=fault-injection/ { fault_selected=$0; next }
  /Scenario result: id=fault-injection/ { print; print fault_selected; next }
  { print }
' "${valid_log}" >"${out_of_order_log}"
malformed_log="${TEST_DIR}/malformed.log"
sed 's/^Scenario selected:/junk Scenario selected:/' "${valid_log}" >"${malformed_log}"
wrong_scenario_log="${TEST_DIR}/wrong-scenario.log"
awk '
  /Scenario selected: id=success/,/Cost evidence:/ { next }
  { print }
' "${valid_log}" >"${wrong_scenario_log}"
cross_scenario_log="${TEST_DIR}/cross-scenario.log"
awk '
  /Scenario selected: id=fault-injection/ { fault_selected=$0; next }
  /Cost evidence: annotation_usd=0.001/ { print fault_selected; print; next }
  { print }
' "${valid_log}" >"${cross_scenario_log}"
integer_reduction_log="${TEST_DIR}/integer-reduction.log"
sed 's/reduction=50.0/reduction=50/' "${valid_log}" >"${integer_reduction_log}"
zero_cost_log="${TEST_DIR}/zero-cost.log"
sed -E 's/annotation_usd=[0-9.]+ metric_usd=[0-9.]+/annotation_usd=0 metric_usd=0/g' "${valid_log}" >"${zero_cost_log}"

for invalid_log in \
  "${missing_log}" \
  "${duplicate_log}" \
  "${out_of_order_log}" \
  "${malformed_log}" \
  "${wrong_scenario_log}" \
  "${cross_scenario_log}" \
  "${integer_reduction_log}" \
  "${zero_cost_log}"; do
  if "${RECORDER}" --scenario all --verify-log "${invalid_log}" >/dev/null 2>&1; then
    printf 'recorder accepted invalid all-scenario log: %s\n' "${invalid_log}" >&2
    exit 1
  fi
done

for signal_status in 130 143; do
  set +e
  bash -c 'source "$1"; handle_signal "$2"' _ "${RECORDER}" "${signal_status}" >/dev/null 2>&1
  exit_code=$?
  set -e
  if [[ ${exit_code} -ne ${signal_status} ]]; then
    printf 'recorder signal handler returned %s, expected %s\n' "${exit_code}" "${signal_status}" >&2
    exit 1
  fi
done

printf 'end-to-end recorder smoke: PASS\n'
