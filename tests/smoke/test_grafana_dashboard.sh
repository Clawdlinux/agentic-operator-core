#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
cd "${REPO_ROOT}"

DASHBOARD="config/grafana/dashboards/cost-by-workload.json"
METRIC="ninevigil_agent_cost_dollars"
PASS=0
FAIL=0

pass() {
  echo "PASS: $*"
  PASS=$((PASS + 1))
}

fail() {
  echo "FAIL: $*" >&2
  FAIL=$((FAIL + 1))
}

if python3 -c "import json; json.load(open('${DASHBOARD}'))"; then
  pass "dashboard JSON is parseable"
else
  fail "dashboard JSON is not parseable"
fi

if grep -q "${METRIC}" "${DASHBOARD}"; then
  pass "dashboard references ${METRIC}"
else
  fail "dashboard does not reference ${METRIC}"
fi

if python3 - "${DASHBOARD}" <<'PY'
import json
import re
import sys
from pathlib import Path

dashboard = json.loads(Path(sys.argv[1]).read_text())
errors = []
grouped_cost_query_found = False
expected_labels = {"workload", "namespace", "model"}
for panel in dashboard.get("panels", []):
    datasource = panel.get("datasource", {})
    targets = panel.get("targets", [])
    if datasource.get("type") != "prometheus":
        continue
    if datasource.get("uid") != "${DS_PROMETHEUS}":
        errors.append(f"panel {panel.get('title')!r} uses datasource {datasource!r}")
    for target in targets:
        expr = target.get("expr", "")
        if "ninevigil_agent_cost_dollars" not in expr:
            continue
        match = re.search(r"by\s*\(([^)]*)\)", expr)
        if not match:
          continue
        labels = {label.strip() for label in match.group(1).split(",") if label.strip()}
        if labels != expected_labels:
          errors.append(f"panel {panel.get('title')!r} groups cost by labels {sorted(labels)!r}")
          continue
        grouped_cost_query_found = True
        legend = target.get("legendFormat", "")
        for label in expected_labels:
            if f"{{{{{label}}}}}" not in legend:
                errors.append(f"panel {panel.get('title')!r} missing {label} in legend {legend!r}")
if not any(
    "ninevigil_agent_cost_dollars" in target.get("expr", "")
    for panel in dashboard.get("panels", [])
    for target in panel.get("targets", [])
):
    errors.append("no Prometheus target references ninevigil_agent_cost_dollars")
if not grouped_cost_query_found:
  errors.append("no cost target groups by workload, namespace, and model")
if errors:
    for error in errors:
        print(error)
    raise SystemExit(1)
PY
then
  pass "dashboard uses ${METRIC} with workload, namespace, and model labels"
else
  fail "dashboard metric labels are incorrect"
fi

if (( FAIL > 0 )); then
  echo "Grafana dashboard smoke test failed: ${FAIL} failed, ${PASS} passed" >&2
  exit 1
fi

echo "Grafana dashboard smoke test passed: ${PASS} checks"
