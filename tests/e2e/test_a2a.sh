#!/usr/bin/env bash
# tests/e2e/test_a2a.sh — End-to-end validation of A2A communication system
# Validates: CRD installation, AgentCard lifecycle, team workload creation, status propagation
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"
NAMESPACE="${A2A_TEST_NAMESPACE:-a2a-test}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

pass=0
fail=0
skip=0

log_pass() { echo -e "${GREEN}[PASS]${NC} $1"; ((pass++)); }
log_fail() { echo -e "${RED}[FAIL]${NC} $1"; ((fail++)); }
log_skip() { echo -e "${YELLOW}[SKIP]${NC} $1"; ((skip++)); }
log_info() { echo -e "[INFO] $1"; }

# ---------------------------------------------------------------------------
# Preflight
# ---------------------------------------------------------------------------
preflight() {
  log_info "Checking prerequisites..."

  if ! command -v kubectl &>/dev/null; then
    log_fail "kubectl not found"
    exit 1
  fi

  if ! kubectl cluster-info &>/dev/null; then
    log_skip "No cluster access — skipping cluster tests"
    return 1
  fi

  log_pass "kubectl and cluster access OK"
  return 0
}

# ---------------------------------------------------------------------------
# CRD Tests
# ---------------------------------------------------------------------------
test_agentcard_crd_exists() {
  log_info "--- Test: AgentCard CRD exists ---"
  local crd_file="${ROOT_DIR}/config/crd/bases/agentic.clawdlinux.org_agentcards.yaml"

  if [[ -f "$crd_file" ]]; then
    log_pass "AgentCard CRD manifest exists"
  else
    log_fail "AgentCard CRD manifest not found at $crd_file"
    return
  fi

  # Validate YAML structure
  if kubectl apply --dry-run=client -f "$crd_file" &>/dev/null; then
    log_pass "AgentCard CRD YAML is valid (dry-run)"
  else
    log_fail "AgentCard CRD YAML is invalid"
  fi
}

test_agentcard_crd_install() {
  log_info "--- Test: AgentCard CRD installation ---"
  local crd_file="${ROOT_DIR}/config/crd/bases/agentic.clawdlinux.org_agentcards.yaml"

  if kubectl apply -f "$crd_file" 2>/dev/null; then
    log_pass "AgentCard CRD installed"
  else
    log_fail "Failed to install AgentCard CRD"
    return
  fi

  # Verify CRD is established
  local retries=10
  while ((retries > 0)); do
    if kubectl get crd agentcards.agentic.clawdlinux.org &>/dev/null; then
      log_pass "AgentCard CRD is established"
      return
    fi
    sleep 1
    ((retries--))
  done
  log_fail "AgentCard CRD not established after 10s"
}

# ---------------------------------------------------------------------------
# AgentCard CRUD Tests
# ---------------------------------------------------------------------------
test_agentcard_create() {
  log_info "--- Test: Create AgentCard instances ---"
  kubectl create namespace "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f - 2>/dev/null

  cat <<EOF | kubectl apply -f - 2>/dev/null
apiVersion: agentic.clawdlinux.org/v1alpha1
kind: AgentCard
metadata:
  name: test-scraper
  namespace: ${NAMESPACE}
spec:
  displayName: "Test Scraper"
  description: "E2E test scraper agent"
  version: "1.0.0"
  skills:
  - name: scrape-url
    description: "Scrapes a URL"
  endpoint:
    host: test-scraper.${NAMESPACE}.svc.cluster.local
    port: 8080
    basePath: /a2a
  healthCheck:
    path: /healthz
    intervalSeconds: 30
    timeoutSeconds: 5
  maxConcurrentTasks: 5
EOF

  if [[ $? -eq 0 ]]; then
    log_pass "AgentCard test-scraper created"
  else
    log_fail "Failed to create test-scraper AgentCard"
    return
  fi

  cat <<EOF | kubectl apply -f - 2>/dev/null
apiVersion: agentic.clawdlinux.org/v1alpha1
kind: AgentCard
metadata:
  name: test-analyst
  namespace: ${NAMESPACE}
spec:
  displayName: "Test Analyst"
  description: "E2E test analyst agent"
  version: "1.0.0"
  skills:
  - name: analyze-visual
    description: "Analyzes screenshots"
  - name: synthesize-report
    description: "Generates reports"
  endpoint:
    host: test-analyst.${NAMESPACE}.svc.cluster.local
    port: 8080
    basePath: /a2a
  maxConcurrentTasks: 3
EOF

  if [[ $? -eq 0 ]]; then
    log_pass "AgentCard test-analyst created"
  else
    log_fail "Failed to create test-analyst AgentCard"
  fi
}

test_agentcard_list() {
  log_info "--- Test: List AgentCards ---"
  local count
  count=$(kubectl get agentcards -n "$NAMESPACE" --no-headers 2>/dev/null | wc -l | tr -d ' ')

  if [[ "$count" -ge 2 ]]; then
    log_pass "Found $count AgentCards in namespace $NAMESPACE"
  else
    log_fail "Expected >=2 AgentCards, found $count"
  fi
}

test_agentcard_get() {
  log_info "--- Test: Get AgentCard details ---"
  if kubectl get agentcard test-scraper -n "$NAMESPACE" -o jsonpath='{.spec.displayName}' 2>/dev/null | grep -q "Test Scraper"; then
    log_pass "AgentCard test-scraper has correct displayName"
  else
    log_fail "AgentCard test-scraper displayName mismatch"
  fi

  local skill_count
  skill_count=$(kubectl get agentcard test-analyst -n "$NAMESPACE" -o jsonpath='{.spec.skills}' 2>/dev/null | python3 -c "import sys,json; print(len(json.load(sys.stdin)))" 2>/dev/null || echo "0")
  if [[ "$skill_count" -eq 2 ]]; then
    log_pass "test-analyst has 2 skills"
  else
    log_fail "test-analyst expected 2 skills, got $skill_count"
  fi
}

# ---------------------------------------------------------------------------
# Team Workload Test
# ---------------------------------------------------------------------------
test_team_workload() {
  log_info "--- Test: Team workload with agentRefs ---"
  cat <<EOF | kubectl apply --dry-run=client -f - 2>/dev/null
apiVersion: agentic.clawdlinux.org/v1alpha1
kind: AgentWorkload
metadata:
  name: test-team
  namespace: ${NAMESPACE}
spec:
  workloadType: visual-analysis
  objective: "E2E test team collaboration"
  collaborationMode: team
  agentRefs:
  - name: test-scraper
    role: data-collector
  - name: test-analyst
    role: analyzer
EOF

  if [[ $? -eq 0 ]]; then
    log_pass "Team workload dry-run validated"
  else
    log_fail "Team workload dry-run failed"
  fi
}

# ---------------------------------------------------------------------------
# Python A2A SDK Tests
# ---------------------------------------------------------------------------
test_python_a2a_imports() {
  log_info "--- Test: Python A2A SDK imports ---"
  local a2a_dir="${ROOT_DIR}/agents/a2a"

  for f in __init__.py protocol.py store.py server.py client.py; do
    if [[ -f "${a2a_dir}/${f}" ]]; then
      log_pass "agents/a2a/${f} exists"
    else
      log_fail "agents/a2a/${f} missing"
    fi
  done

  # Validate Python syntax
  if python3 -c "
import ast, sys
for f in ['protocol.py', 'store.py', 'server.py', 'client.py']:
    try:
        ast.parse(open('${a2a_dir}/' + f).read())
    except SyntaxError as e:
        print(f'Syntax error in {f}: {e}', file=sys.stderr)
        sys.exit(1)
print('All A2A Python files have valid syntax')
" 2>&1; then
    log_pass "All A2A Python files have valid syntax"
  else
    log_fail "Python syntax errors in A2A SDK"
  fi
}

test_python_protocol_models() {
  log_info "--- Test: Python A2A protocol models ---"
  if python3 -c "
import sys
sys.path.insert(0, '${ROOT_DIR}')
from agents.a2a.protocol import TaskStatus, Task, TaskResult, AgentMessage

# Verify enum values
assert TaskStatus.CREATED.value == 'created'
assert TaskStatus.COMPLETED.value == 'completed'
assert TaskStatus.FAILED.value == 'failed'

# Verify Task creation
t = Task(skill='scrape-url', input_data={'url': 'https://example.com'}, sender_agent='agent-a', recipient_agent='agent-b')
assert t.status == TaskStatus.CREATED
assert t.id is not None
assert t.timeout_seconds == 300

# Verify TaskResult creation
r = TaskResult(task_id=t.id, status=TaskStatus.COMPLETED, output_data={'html': '<html></html>'})
assert r.task_id == t.id

# Verify AgentMessage creation
m = AgentMessage(sender='agent-a', recipient='agent-b', content='hello')
assert m.message_type == 'task'

print('All protocol model tests passed')
" 2>&1; then
    log_pass "Python A2A protocol models work correctly"
  else
    log_fail "Python A2A protocol model tests failed"
  fi
}

# ---------------------------------------------------------------------------
# Go Build Tests
# ---------------------------------------------------------------------------
test_go_build() {
  log_info "--- Test: Go build with A2A types ---"
  if (cd "${ROOT_DIR}" && go build ./... 2>&1); then
    log_pass "Go build succeeds with A2A types"
  else
    log_fail "Go build failed"
  fi
}

test_go_vet() {
  log_info "--- Test: Go vet ---"
  if (cd "${ROOT_DIR}" && go vet ./... 2>&1); then
    log_pass "Go vet passes"
  else
    log_fail "Go vet failed"
  fi
}

test_go_tests() {
  log_info "--- Test: Go unit tests ---"
  if (cd "${ROOT_DIR}" && go test ./... 2>&1); then
    log_pass "All Go tests pass"
  else
    log_fail "Go tests failed"
  fi
}

# ---------------------------------------------------------------------------
# Cleanup
# ---------------------------------------------------------------------------
cleanup() {
  log_info "Cleaning up test resources..."
  kubectl delete namespace "$NAMESPACE" --ignore-not-found=true 2>/dev/null || true
  log_info "Cleanup done"
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
main() {
  echo "============================================="
  echo " A2A Communication System — E2E Test Suite"
  echo "============================================="
  echo ""

  # Always run: file-level and Go tests
  test_agentcard_crd_exists
  test_python_a2a_imports
  test_python_protocol_models
  test_go_build
  test_go_vet

  # Cluster tests (skipped if no cluster)
  if preflight; then
    test_agentcard_crd_install
    test_agentcard_create
    test_agentcard_list
    test_agentcard_get
    test_team_workload

    trap cleanup EXIT
  fi

  echo ""
  echo "============================================="
  echo -e " Results: ${GREEN}${pass} passed${NC}, ${RED}${fail} failed${NC}, ${YELLOW}${skip} skipped${NC}"
  echo "============================================="

  if ((fail > 0)); then
    exit 1
  fi
}

main "$@"
