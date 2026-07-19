#!/usr/bin/env bash
set -euo pipefail
umask 077

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "${ROOT_DIR}"

PACE_SECONDS="${DEMO_RECORDING_PACE_SECONDS:-3}"
DURATION_SECONDS="${DEMO_RECORDING_DURATION_SECONDS:-120}"
START_DELAY_SECONDS="${DEMO_RECORDING_START_DELAY_SECONDS:-12}"
SCENARIO="${DEMO_RECORDING_SCENARIO:-all}"
PORT="${DEMO_VIZ_PORT:-8765}"
OUTPUT="${DEMO_RECORDING_OUTPUT:-}"
DRY_RUN=false
OPEN_RECORDING=true
VERIFY_LOG_ONLY=""
VISUALIZER_PID=""

usage() {
  cat <<EOF
Usage: $(basename "$0") [OPTIONS]

Records the real booth script and live dashboard as a local WebM file.

Options:
  --pace SECONDS       Demo narration pause. Default: ${PACE_SECONDS}
  --scenario NAME      success, fault-injection, or all. Default: ${SCENARIO}
  --duration SECONDS   Recording window, 20-180 seconds. Default: ${DURATION_SECONDS}
  --start-delay SEC    Command pre-roll, 5-30 seconds. Default: ${START_DELAY_SECONDS}
  --output PATH        Local .webm path. Default: demos/local/clawdlinux-e2e-TIMESTAMP.webm
  --verify-log PATH    Verify an existing recorder log and exit.
  --no-open            Do not open the completed recording.
  --dry-run            Print the recording plan without starting apps.
  -h, --help           Show this help.

The WebM and stdout log are generated under demos/local/ and ignored by Git.
EOF
}

die() {
  printf '[FAIL] %s\n' "$*" >&2
  exit 1
}

validate_integer() {
  local name="$1"
  local value="$2"
  local minimum="$3"
  local maximum="$4"
  [[ "${value}" =~ ^[0-9]+$ ]] || die "${name} must be an integer from ${minimum} to ${maximum}"
  ((10#${value} >= minimum && 10#${value} <= maximum)) ||
    die "${name} must be an integer from ${minimum} to ${maximum}"
}

validate_scenario() {
  case "$1" in
    success|fault-injection|all) ;;
    *) die "scenario must be success, fault-injection, or all" ;;
  esac
}

verify_recorded_log() {
  local log_file="$1"
  [[ -f "${log_file}" ]] || die "recorded log is not readable: ${log_file}"
  python3 - "${log_file}" "${SCENARIO}" <<'PY' || die "recorded log did not satisfy the ${SCENARIO} scenario contract"
import re
import sys
from decimal import Decimal

log_path, requested = sys.argv[1:]
scenarios = {
  "success": {
    "expected": "Complete",
    "fixture": "examples/booth-scenarios/success/fixture.yaml",
    "workload": "examples/booth-scenarios/success/workload.template.yaml",
  },
  "fault-injection": {
    "expected": "Failed",
    "fixture": "examples/booth-scenarios/fault-injection/fixture.yaml",
    "workload": "examples/booth-scenarios/fault-injection/workload.template.yaml",
  },
}
ordered_ids = ["success", "fault-injection"] if requested == "all" else [requested]

dns_label = r"[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?"
cluster_id = r"[A-Za-z0-9](?:[A-Za-z0-9._-]{0,126}[A-Za-z0-9])?"
uint = r"[0-9]{1,10}"
cost_decimal = r"(?:0|[1-9][0-9]{0,5})(?:\.[0-9]{1,9})?"
anf = re.compile(
  rf"^ANF context: source=kubernetes/{cluster_id} "
  rf"scope=namespace:{dns_label} "
  rf"source_bytes={uint} source_objects={uint} projected_objects={uint} "
  rf"unprojected_pods={uint} omitted_containers={uint} "
  rf"omitted_service_ports={uint} omitted_named_target_ports={uint} "
  rf"document_json_bytes={uint} anf_bytes={uint} "
  rf"document_json_tokens_est={uint} anf_tokens_est={uint} "
  rf"reduction=-?[0-9]{{1,10}}\.[0-9] top_level_entities={uint}$"
)
provider = re.compile(
  r"^Provider result: gateway=litellm route=litellm/clawdlinux-anthropic "
  rf"provider=claude input_tokens={uint} output_tokens={uint}$"
)
cost = re.compile(
  rf"^Cost evidence: annotation_usd=(?P<annotation>{cost_decimal}) "
  rf"metric_usd=(?P<metric>{cost_decimal}) route=litellm/clawdlinux-anthropic$"
)

def valid_provider(line):
  match = provider.fullmatch(line)
  if match is None:
    return False
  values = [int(value) for value in re.findall(r"(?:input|output)_tokens=([0-9]+)", line)]
  return len(values) == 2 and all(value > 0 for value in values)

def valid_cost(line):
  match = cost.fullmatch(line)
  return (
    match is not None
    and Decimal(match.group("annotation")) > 0
    and Decimal(match.group("metric")) > 0
  )

expected = []
for scenario_id in ordered_ids:
  scenario = scenarios[scenario_id]
  expected.extend([
    (
      "selection",
      lambda line, scenario_id=scenario_id, scenario=scenario: line == (
        f"Scenario selected: id={scenario_id} expected={scenario['expected']} "
        f"fixture={scenario['fixture']} workload={scenario['workload']}"
      ),
    ),
    (
      "result",
      lambda line, scenario_id=scenario_id, scenario=scenario: line == (
        f"Scenario result: id={scenario_id} expected={scenario['expected']} "
        f"observed={scenario['expected']}"
      ),
    ),
    ("ANF", lambda line: anf.fullmatch(line) is not None),
    ("completion", lambda line: line == "[OK] AgentWorkload reached Completed"),
    ("provider", valid_provider),
    ("cost", valid_cost),
  ])
expected.extend([
  ("tamper", lambda line: line == "[OK] Tampered prior-run artifact was rejected"),
  ("summary", lambda line: line == "CURRENT --present EVIDENCE"),
  ("finish", lambda line: line == "Run finished. Dashboard server stays up -- Ctrl+C to stop."),
])

reserved = (
  "Scenario selected:",
  "Scenario result:",
  "ANF context:",
  "[OK] AgentWorkload reached Completed",
  "Provider result:",
  "Cost evidence:",
  "[OK] Tampered prior-run artifact was rejected",
  "CURRENT --present EVIDENCE",
  "Run finished.",
)
with open(log_path, encoding="utf-8") as source:
  contracts = [line.rstrip("\r\n") for line in source if line.startswith(reserved)]

if len(contracts) != len(expected):
  raise SystemExit(f"contract count {len(contracts)} does not equal {len(expected)}")
for index, (line, (name, matches)) in enumerate(zip(contracts, expected), start=1):
  if not matches(line):
    raise SystemExit(f"contract {index} does not match expected {name}")
PY
}

cleanup() {
  local exit_code=$?
  trap - EXIT INT TERM
  if [[ -n "${VISUALIZER_PID}" ]]; then
    kill "${VISUALIZER_PID}" >/dev/null 2>&1 || true
    wait "${VISUALIZER_PID}" >/dev/null 2>&1 || true
  fi
  return "${exit_code}"
}

handle_signal() {
  trap - INT TERM
  exit "$1"
}

main() {

while (($#)); do
  case "$1" in
    --pace)
      [[ $# -ge 2 ]] || die "--pace requires a value"
      PACE_SECONDS="$2"
      shift 2
      ;;
    --scenario)
      [[ $# -ge 2 ]] || die "--scenario requires a value"
      SCENARIO="$2"
      shift 2
      ;;
    --duration)
      [[ $# -ge 2 ]] || die "--duration requires a value"
      DURATION_SECONDS="$2"
      shift 2
      ;;
    --start-delay)
      [[ $# -ge 2 ]] || die "--start-delay requires a value"
      START_DELAY_SECONDS="$2"
      shift 2
      ;;
    --output)
      [[ $# -ge 2 ]] || die "--output requires a value"
      OUTPUT="$2"
      shift 2
      ;;
    --verify-log)
      [[ $# -ge 2 ]] || die "--verify-log requires a value"
      VERIFY_LOG_ONLY="$2"
      shift 2
      ;;
    --no-open)
      OPEN_RECORDING=false
      shift
      ;;
    --dry-run)
      DRY_RUN=true
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

validate_integer "pace" "${PACE_SECONDS}" 0 60
validate_scenario "${SCENARIO}"
validate_integer "duration" "${DURATION_SECONDS}" 20 180
validate_integer "start delay" "${START_DELAY_SECONDS}" 5 30
validate_integer "DEMO_VIZ_PORT" "${PORT}" 1024 65535

if [[ -n "${VERIFY_LOG_ONLY}" ]]; then
  verify_recorded_log "${VERIFY_LOG_ONLY}"
  printf 'Recorded log verification: PASS\n'
  exit 0
fi

if [[ -z "${OUTPUT}" ]]; then
  OUTPUT="demos/local/clawdlinux-e2e-$(date +%Y%m%dT%H%M%S).webm"
fi
[[ "${OUTPUT}" == *.webm ]] || die "output path must end in .webm"
if [[ "${OUTPUT}" != /* ]]; then
  OUTPUT="${ROOT_DIR}/${OUTPUT}"
fi
LOG_FILE="${OUTPUT%.webm}.log"

VISUALIZER_COMMAND=(
  python3 -u scripts/demo-visualizer.py
  --present
  --scenario "${SCENARIO}"
  --tamper-audit
  --pace "${PACE_SECONDS}"
)

printf 'Source command: '
printf '%q ' "${VISUALIZER_COMMAND[@]}"
printf '\nDashboard: http://127.0.0.1:%s\nRecording: %s\n' "${PORT}" "${OUTPUT}"

if [[ "${DRY_RUN}" == "true" ]]; then
  printf 'Dry run complete. No apps or recording were started.\n'
  exit 0
fi

for command in python3 node npm; do
  command -v "${command}" >/dev/null 2>&1 || die "missing required command: ${command}"
done

CHROME_BIN="${DEMO_RECORDING_CHROME:-/Applications/Google Chrome.app/Contents/MacOS/Google Chrome}"
[[ -x "${CHROME_BIN}" ]] || die "Google Chrome is not available at ${CHROME_BIN}"
PLAYWRIGHT_ROOT="$(npm root -g)"
NODE_PATH="${PLAYWRIGHT_ROOT}" node -e 'require("playwright")' >/dev/null 2>&1 ||
  die "global Playwright is required: npm install -g playwright"
PLAYWRIGHT_FFMPEG=""
for cache_directory in \
  "${HOME}/Library/Caches/ms-playwright" \
  "${HOME}/.cache/ms-playwright"; do
  if [[ -d "${cache_directory}" ]]; then
    PLAYWRIGHT_FFMPEG="$(
      find "${cache_directory}" -type f -path '*/ffmpeg-*/*' -perm -111 -print -quit
    )"
    [[ -z "${PLAYWRIGHT_FFMPEG}" ]] || break
  fi
done
[[ -n "${PLAYWRIGHT_FFMPEG}" ]] ||
  die "Playwright FFmpeg is required: playwright install ffmpeg"

python3 - "${PORT}" <<'PY' || die "dashboard port is already in use"
import socket
import sys

port = int(sys.argv[1])
with socket.socket() as server:
    server.bind(("127.0.0.1", port))
PY

mkdir -p "$(dirname "${OUTPUT}")"

trap cleanup EXIT
trap 'handle_signal 130' INT
trap 'handle_signal 143' TERM

DEMO_VIZ_PORT="${PORT}" \
DEMO_START_DELAY_SECONDS="${START_DELAY_SECONDS}" \
  "${VISUALIZER_COMMAND[@]}" >"${LOG_FILE}" 2>&1 &
VISUALIZER_PID=$!

if ! python3 - "http://127.0.0.1:${PORT}" "${VISUALIZER_PID}" <<'PY'
import os
import sys
import time
import urllib.request

url = sys.argv[1]
pid = int(sys.argv[2])
for _ in range(100):
    try:
        with urllib.request.urlopen(url, timeout=0.2) as response:
            if response.status == 200:
                raise SystemExit(0)
    except Exception:
        pass
    try:
        os.kill(pid, 0)
    except OSError:
        raise SystemExit(1)
    time.sleep(0.1)
raise SystemExit(1)
PY
then
  cat "${LOG_FILE}" >&2
  die "visualizer did not become ready"
fi

printf 'Recording the dashboard for up to %ss.\n' "${DURATION_SECONDS}"
NODE_PATH="${PLAYWRIGHT_ROOT}" node scripts/record-dashboard.js \
  "http://127.0.0.1:${PORT}" \
  "${OUTPUT}" \
  "${DURATION_SECONDS}" \
  "${CHROME_BIN}"

[[ -s "${OUTPUT}" ]] || die "screen recording was not created"
verify_recorded_log "${LOG_FILE}"

ln -sfn "$(basename "${OUTPUT}")" "$(dirname "${OUTPUT}")/latest.webm"
ln -sfn "$(basename "${LOG_FILE}")" "$(dirname "${LOG_FILE}")/latest.log"

printf 'End-to-end recording complete.\nWebM: %s\nLog: %s\nLatest: %s\n' \
  "${OUTPUT}" \
  "${LOG_FILE}" \
  "$(dirname "${OUTPUT}")/latest.webm"

if [[ "${OPEN_RECORDING}" == "true" ]]; then
  open "${OUTPUT}"
fi
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  main "$@"
fi
