# Week 4 Argo Integration: Strategic Plan

**Created:** 2026-02-23  
**Status:** Ready for Implementation  
**Author:** Goodra (Architecture Review)

---

## Executive Summary

Week 4 integrates the Go operator (Weeks 1-3, ~3,500 LOC) with Argo Workflows to orchestrate visual competitor scraping. The architecture is **sound** with manageable risks. This document provides implementation order, integration points, testing strategy, and go/no-go criteria.

**Verdict: GO** — with the mitigation strategies outlined below.

---

## 1. Architecture Validation

### 1.1 Proposed Architecture

```
AgentWorkload CR (apply)
       │
       ▼
┌─────────────────────────────┐
│  Go Operator Reconciler     │
│  (agentworkload_controller) │
└─────────────────────────────┘
       │
       ├─── OPA Evaluation (existing)
       │
       ├─── createArgoWorkflow() [NEW]
       │         │
       │         ▼
       │    Argo Workflow CR
       │         │
       │         ▼
       │   ┌─────────────────────────────────────────────┐
       │   │  Argo WorkflowTemplate                      │
       │   │  ┌──────────┐    ┌──────────────────────┐  │
       │   │  │ Scraper  │───▶│ Screenshot ║ DOM     │  │
       │   │  │ (Step 1) │    │ (Parallel - Step 2)  │  │
       │   │  └──────────┘    └──────────────────────┘  │
       │   │                           │                 │
       │   │                           ▼                 │
       │   │                  ┌────────────────┐        │
       │   │                  │ Suspend Gate   │        │
       │   │                  │ (OPA approval) │        │
       │   │                  └────────────────┘        │
       │   │                           │                 │
       │   │                           ▼                 │
       │   │                  ┌────────────────┐        │
       │   │                  │ Synthesis      │        │
       │   │                  │ (Step 4)       │        │
       │   │                  └────────────────┘        │
       │   └─────────────────────────────────────────────┘
       │
       └─── resumeArgoWorkflow() [NEW]
                  │
                  ▼
            MinIO Artifact Output
```

### 1.2 Design Assessment: ✅ SOUND

| Aspect | Assessment | Rationale |
|--------|------------|-----------|
| **Separation of Concerns** | ✅ Good | Go operator handles CRD/OPA, Argo handles workflow execution, Python handles agent logic |
| **Durability** | ✅ Good | LangGraph checkpoints to PostgreSQL; Argo has native retry/resumption |
| **Observability** | ✅ Good | Argo UI shows DAG execution; operator logs reconciliation |
| **Testability** | ✅ Good | Each layer testable in isolation (unit → integration → E2E) |
| **Complexity** | ⚠️ Moderate | 4 moving parts (operator, Argo, agents, MinIO) — manageable for solo founder |

### 1.3 Design Risks & Mitigations

| Risk | Severity | Mitigation |
|------|----------|------------|
| **Argo version mismatch** | Medium | Pin to v4.0.1, test on kind before prod |
| **Workflow CR ownership** | High | Use `ownerReferences` → AgentWorkload deletion cascades to Workflow |
| **Suspend gate deadlock** | Medium | Add timeout (30m) + alerting; operator can force-resume |
| **MinIO connectivity from pods** | Medium | Use K8s Service DNS (`minio.shared-services.svc.cluster.local`) |
| **Python agent image not built** | High | Pre-build image, push to registry before E2E |
| **LangGraph checkpoint DB connection** | Medium | PostgreSQL must be deployed before agents run |

---

## 2. Integration Points

### 2.1 Go Operator → Argo Workflows

**New code required in `agentworkload_controller.go`:**

```go
// pkg/argo/workflow.go (NEW FILE)

func (r *AgentWorkloadReconciler) createArgoWorkflow(
    ctx context.Context, 
    workload *agenticv1alpha1.AgentWorkload,
) (*unstructured.Unstructured, error) {
    // 1. Build Workflow from WorkflowTemplate
    // 2. Set ownerReference to AgentWorkload
    // 3. Pass parameters (target_urls, job_id, minio_bucket)
    // 4. Create via dynamic client
}

func (r *AgentWorkloadReconciler) resumeArgoWorkflow(
    ctx context.Context,
    workflowName string,
) error {
    // PATCH workflow to resume from suspend node
    // Argo API: POST /api/v1/workflows/{namespace}/{name}/resume
}
```

**Integration points:**
1. **Create Workflow** — After OPA approves initial action, operator creates Argo Workflow CR
2. **Watch Workflow** — Operator watches Workflow status changes
3. **Resume Workflow** — When OPA approves suspended step, operator resumes Workflow
4. **Read Artifacts** — On completion, operator reads MinIO artifact path from Workflow output

### 2.2 Failure Modes

| Failure | Detection | Recovery |
|---------|-----------|----------|
| Argo controller not running | Workflow stuck in `Pending` | Operator sets `AgentWorkload.status.phase = "Failed"` after 5m timeout |
| Workflow step fails | Argo reports `status.phase = "Failed"` | Operator propagates to AgentWorkload status; retry policy in template |
| Suspend gate timeout | Workflow suspended > 30m | Operator auto-denies, marks AgentWorkload as `TimedOut` |
| MinIO unavailable | Artifact upload fails | Retry 3x with backoff; fail workflow if persistent |
| Browserless unavailable | Scraper step fails | Argo retry (maxRetries: 2, backoff); circuit breaker in Python code |

### 2.3 RBAC Requirements

The operator needs additional RBAC to create/watch Argo Workflows:

```yaml
# config/rbac/argo_role.yaml (NEW)
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: agentworkload-argo-manager
rules:
  - apiGroups: ["argoproj.io"]
    resources: ["workflows", "workflowtemplates"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  - apiGroups: ["argoproj.io"]
    resources: ["workflows/status"]
    verbs: ["get", "watch"]
```

---

## 3. Dependency Graph

### 3.1 Task Dependencies

```
4A: Argo Install ─────────────────────────────┐
                                              │
4D: MinIO + Browserless ──────────────────────┼───▶ 4E: E2E Test
                                              │
4F: LiteLLM Proxy ────────────────────────────┤
                                              │
4B: WorkflowTemplate ─────────────────────────┤
                                              │
4C: Operator Integration ─────────────────────┘
```

### 3.2 Parallelization Opportunities

| Can Run in Parallel | Reason |
|---------------------|--------|
| 4A + 4D + 4F | Infrastructure deployments are independent |
| 4B (early design) | Template YAML can be drafted while infra deploys |
| 4C (skeleton) | Go code skeleton can be written while waiting for infra |

| Must Be Sequential | Reason |
|--------------------|--------|
| 4A → 4B → 4E | Need Argo installed to test templates |
| 4D → 4E | Need MinIO/Browserless to run E2E |
| 4C → 4E | Need operator integration to test full pipeline |

---

## 4. Testing Strategy

### 4.1 Test Pyramid

```
          ┌─────────────────┐
          │   E2E (4E)      │  ← 1-2 tests, full pipeline
          │   ~15 minutes   │
          └────────┬────────┘
                   │
          ┌────────▼────────┐
          │  Integration    │  ← 5-10 tests, component pairs
          │   ~5 minutes    │
          └────────┬────────┘
                   │
          ┌────────▼────────┐
          │     Unit        │  ← 20+ tests, isolated logic
          │   ~30 seconds   │
          └─────────────────┘
```

### 4.2 Test Plan by Layer

#### Unit Tests (pkg/argo/*)

| Test | What It Validates |
|------|-------------------|
| `TestBuildWorkflowFromTemplate` | Workflow YAML generation with correct parameters |
| `TestOwnerReferenceSet` | Cascade deletion configured correctly |
| `TestResumeWorkflowRequest` | API payload format for resume |
| `TestWorkflowStatusParsing` | Status extraction from Workflow CR |

#### Integration Tests (internal/controller/*_test.go)

| Test | What It Validates |
|------|-------------------|
| `TestReconciler_CreatesArgoWorkflow` | Operator creates Workflow when AgentWorkload applied |
| `TestReconciler_WatchesWorkflowStatus` | Operator updates AgentWorkload status from Workflow |
| `TestReconciler_ResumesOnApproval` | Operator resumes Workflow when OPA approves |
| `TestReconciler_FailsOnWorkflowTimeout` | Operator fails AgentWorkload after Workflow timeout |

#### E2E Test (4E)

**Single comprehensive test:**

```gherkin
Scenario: Full competitor scraping pipeline
  Given kind cluster with Argo, MinIO, Browserless, LiteLLM
  When I apply AgentWorkload CR with target_urls ["https://example.com"]
  Then Argo Workflow should be created
  And Scraper step should complete
  And Screenshot + DOM steps should run in parallel
  And Workflow should suspend at gate
  When I approve via operator API
  Then Workflow should resume
  And Synthesis step should complete
  And MinIO should contain report artifact
  And AgentWorkload status.phase should be "Completed"
```

### 4.3 Test Environment

```yaml
# kind cluster config
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
  - role: control-plane
  - role: worker
    extraPortMappings:
      - containerPort: 30080  # Argo UI
        hostPort: 30080
      - containerPort: 30090  # MinIO console
        hostPort: 30090
```

---

## 5. Blocker Identification

### 5.1 Critical Blockers (Must Fix Before Start)

| Blocker | Status | Resolution |
|---------|--------|------------|
| **Python agent Docker image** | ❌ Not built | Create `Dockerfile` in `agents/`, build + push to registry |
| **PostgreSQL for LangGraph** | ❌ Not deployed | Add PostgreSQL to `shared-services` namespace |
| **Argo Workflows CRD not in go.mod** | ❌ Missing | `go get github.com/argoproj/argo-workflows/v3@v3.4.0` |
| **Dynamic client for unstructured resources** | ❓ May need | Add `k8s.io/client-go/dynamic` for Workflow CR creation |

### 5.2 Potential Blockers (Monitor During Implementation)

| Potential Issue | Detection | Mitigation |
|-----------------|-----------|------------|
| Argo 4.0.1 not stable on kind | Workflow controller crashes | Fall back to 3.5.x LTS |
| MinIO credentials management | Auth failures | Use K8s Secret + ServiceAccount |
| Browserless Chrome memory | OOMKilled pods | Set resource limits, use headless mode |
| LiteLLM proxy rate limiting | 429 errors | Add retry with exponential backoff |

### 5.3 Code Gaps Analysis

**Existing code that needs modification:**

| File | Change Required |
|------|-----------------|
| `agentworkload_controller.go` | Add Argo Workflow create/watch/resume logic |
| `agentworkload_types.go` | Add `status.argoWorkflowName`, `status.argoWorkflowPhase` |
| `agents/entrypoint.py` | Add MinIO upload on completion |
| `agents/tools/litellm_client.py` | Configure proxy endpoint from env var |

**New code required:**

| File | Purpose |
|------|---------|
| `pkg/argo/workflow.go` | Argo Workflow builder + resume logic |
| `pkg/argo/workflow_test.go` | Unit tests |
| `config/argo/workflowtemplate.yaml` | WorkflowTemplate CRD |
| `config/shared-services/minio.yaml` | MinIO deployment |
| `config/shared-services/browserless.yaml` | Browserless deployment |
| `config/shared-services/litellm.yaml` | LiteLLM proxy deployment |
| `config/shared-services/postgres.yaml` | PostgreSQL for LangGraph |
| `agents/Dockerfile` | Python agent container image |
| `scripts/setup-kind.sh` | kind cluster + Argo install automation |
| `tests/e2e/e2e_test.go` | E2E test suite |

---

## 6. Implementation Order

### Recommended Sequence (differs from 4A-4F)

**Phase 1: Foundation (Day 1-2)**

```
┌─────────────────────────────────────────────────────┐
│ 1.1 Create kind cluster setup script               │
│     - scripts/setup-kind.sh                        │
│     - Verify kind works locally                    │
│                                                     │
│ 1.2 Install Argo Workflows (4A)                   │
│     - kubectl apply -f argo-workflows-v4.0.1.yaml │
│     - Verify Argo controller running               │
│     - Access Argo UI at localhost:30080           │
│                                                     │
│ 1.3 Deploy shared-services namespace              │
│     - MinIO (4D)                                   │
│     - PostgreSQL (for LangGraph)                   │
│     - Verify connectivity                          │
└─────────────────────────────────────────────────────┘
```

**Phase 2: Agent Containerization (Day 2-3)**

```
┌─────────────────────────────────────────────────────┐
│ 2.1 Create agents/Dockerfile                       │
│     - Python 3.11 base                             │
│     - Install langgraph, aiohttp, websockets       │
│     - Copy agents/ code                            │
│                                                     │
│ 2.2 Build + push image                            │
│     - docker build -t agent:latest agents/         │
│     - kind load docker-image agent:latest         │
│                                                     │
│ 2.3 Deploy Browserless (4D continued)             │
│     - browserless/chrome deployment               │
│     - Test WebSocket connectivity                  │
│                                                     │
│ 2.4 Deploy LiteLLM proxy (4F)                     │
│     - LiteLLM with OpenAI backend config          │
│     - Test vision model endpoint                   │
└─────────────────────────────────────────────────────┘
```

**Phase 3: WorkflowTemplate (Day 3-4)**

```
┌─────────────────────────────────────────────────────┐
│ 3.1 Design WorkflowTemplate YAML (4B)              │
│     - 4 steps: scrape → parallel(screenshot, dom)  │
│              → suspend → synthesis                 │
│     - Parameters: target_urls, job_id, minio_path  │
│     - Artifacts: output to MinIO                   │
│                                                     │
│ 3.2 Test template manually                         │
│     - argo submit --from workflowtemplate/scraper │
│     - Verify each step runs                        │
│     - Verify suspend gate works                    │
│     - argo resume <workflow> to test resume        │
└─────────────────────────────────────────────────────┘
```

**Phase 4: Operator Integration (Day 4-5)**

```
┌─────────────────────────────────────────────────────┐
│ 4.1 Add Argo SDK to go.mod                         │
│     - go get github.com/argoproj/argo-workflows/v3 │
│                                                     │
│ 4.2 Implement pkg/argo/workflow.go (4C)           │
│     - createArgoWorkflow()                         │
│     - resumeArgoWorkflow()                         │
│     - watchWorkflowStatus()                        │
│     - Unit tests                                    │
│                                                     │
│ 4.3 Update controller reconciliation              │
│     - Integrate Argo workflow creation             │
│     - Add status watching                          │
│     - Add resume on OPA approval                   │
│     - Integration tests                            │
│                                                     │
│ 4.4 Update CRD types                              │
│     - Add ArgoWorkflowRef to status               │
│     - Add ArgoPhase to status                      │
│     - make manifests                               │
└─────────────────────────────────────────────────────┘
```

**Phase 5: E2E Validation (Day 5-6)**

```
┌─────────────────────────────────────────────────────┐
│ 5.1 Write E2E test (4E)                           │
│     - tests/e2e/e2e_test.go                        │
│     - Setup: kind cluster + all components        │
│     - Test: full pipeline CR → artifact           │
│                                                     │
│ 5.2 Run E2E test suite                            │
│     - make test-e2e                                │
│     - Debug any failures                           │
│     - Verify MinIO artifact exists                 │
│                                                     │
│ 5.3 Code review + cleanup                         │
│     - Review all new code (code-review skill)      │
│     - Update documentation                         │
│     - Tag commit                                   │
└─────────────────────────────────────────────────────┘
```

### Timeline Summary

| Day | Tasks | Deliverable |
|-----|-------|-------------|
| 1 | 1.1, 1.2 | kind + Argo running |
| 2 | 1.3, 2.1, 2.2 | MinIO + agent image |
| 3 | 2.3, 2.4, 3.1 | Browserless + LiteLLM + template design |
| 4 | 3.2, 4.1, 4.2 | Manual workflow test + Argo Go package |
| 5 | 4.3, 4.4 | Operator integration complete |
| 6 | 5.1, 5.2, 5.3 | E2E passing + code review |

---

## 7. Go/No-Go Criteria

### 7.1 Pre-Implementation Checklist

Before starting Week 4:

- [ ] **kind installed locally** — `kind version` returns v0.20+
- [ ] **Docker running** — `docker ps` works
- [ ] **Go 1.21+ installed** — `go version`
- [ ] **kubectl configured** — `kubectl cluster-info` (after kind create)
- [ ] **Argo CLI installed** — `argo version`
- [ ] **Week 1-3 code committed** — All tests passing
- [ ] **Registry access** — Can push/pull images (local kind registry or Docker Hub)

### 7.2 Phase Gate Criteria

**After Phase 1 (Foundation):**
- [ ] `argo list` returns empty list (controller healthy)
- [ ] MinIO console accessible at `localhost:30090`
- [ ] PostgreSQL accepts connections from pod

**After Phase 2 (Containerization):**
- [ ] `agents:latest` image loaded in kind
- [ ] Browserless WebSocket test passes
- [ ] LiteLLM `/health` endpoint returns 200

**After Phase 3 (WorkflowTemplate):**
- [ ] Manual `argo submit` creates workflow
- [ ] All 4 steps complete (with mock data)
- [ ] Suspend gate pauses workflow
- [ ] `argo resume` continues to synthesis

**After Phase 4 (Integration):**
- [ ] `go test ./pkg/argo -v` — 100% pass
- [ ] `go test ./internal/controller -v` — 100% pass
- [ ] Operator creates Workflow on CR apply

**After Phase 5 (E2E):**
- [ ] `make test-e2e` passes
- [ ] MinIO contains report artifact
- [ ] AgentWorkload status shows "Completed"
- [ ] Code review approved

### 7.3 No-Go Signals

**Stop and reassess if:**
- Argo 4.0.1 has critical bugs on kind (fallback: use 3.5.x)
- Python agent fails to connect to services (network policy issue)
- Workflow controller uses >1GB memory (resource constraints)
- Integration adds >500 LOC to operator (complexity budget exceeded)

---

## 8. Files to Create

### 8.1 Directory Structure After Week 4

```
agentic-k8s-operator/
├── agents/
│   ├── Dockerfile                    [NEW]
│   ├── entrypoint.py                 [UPDATE: MinIO upload]
│   └── ...
├── config/
│   ├── argo/
│   │   └── workflowtemplate.yaml     [NEW]
│   ├── shared-services/
│   │   ├── namespace.yaml            [NEW]
│   │   ├── minio.yaml                [NEW]
│   │   ├── browserless.yaml          [NEW]
│   │   ├── litellm.yaml              [NEW]
│   │   └── postgres.yaml             [NEW]
│   └── rbac/
│       └── argo_role.yaml            [NEW]
├── pkg/
│   ├── argo/
│   │   ├── workflow.go               [NEW]
│   │   └── workflow_test.go          [NEW]
│   └── ...
├── scripts/
│   └── setup-kind.sh                 [NEW]
├── tests/
│   └── e2e/
│       └── e2e_test.go               [NEW]
└── docs/
    └── WEEK4_ARGO_STRATEGIC_PLAN.md  [THIS FILE]
```

### 8.2 Estimated LOC by Component

| Component | New LOC | Modified LOC |
|-----------|---------|--------------|
| `pkg/argo/` | ~200 | — |
| `config/argo/` | ~100 | — |
| `config/shared-services/` | ~300 | — |
| `agents/Dockerfile` | ~30 | — |
| `scripts/setup-kind.sh` | ~80 | — |
| `tests/e2e/` | ~150 | — |
| Controller updates | — | ~100 |
| CRD type updates | — | ~20 |
| **Total** | **~860** | **~120** |

**Week 4 adds ~1000 LOC** — Within complexity budget for solo founder.

---

## 9. Risk Register

| ID | Risk | Probability | Impact | Mitigation |
|----|------|-------------|--------|------------|
| R1 | Argo 4.x incompatible with kind | Low | High | Test early; fallback to 3.5.x |
| R2 | Python agent OOMKilled | Medium | Medium | Set memory limits; optimize image |
| R3 | MinIO auth issues | Low | Medium | Use well-documented config |
| R4 | LiteLLM rate limits | Medium | Low | Add retry logic; cache responses |
| R5 | Workflow stuck in Pending | Medium | Medium | Add timeout; alert on long pending |
| R6 | E2E test flaky | High | Medium | Add retries; mock external services |
| R7 | Integration complexity exceeds estimate | Medium | High | Timebox each phase; cut scope if needed |

---

## 10. Success Metrics

At end of Week 4, we should have:

1. **Functional Pipeline:**
   - AgentWorkload CR → Argo Workflow → MinIO artifact
   - Suspend gate + OPA approval working
   - Full DAG execution (scrape → parallel → synthesis)

2. **Test Coverage:**
   - Unit tests: 90%+ for new `pkg/argo/` code
   - Integration tests: All controller flows covered
   - E2E test: 1 comprehensive test passing

3. **Documentation:**
   - WEEK4_SUMMARY.md with implementation notes
   - Updated README with Argo integration section
   - Inline code comments for new functions

4. **Code Quality:**
   - All existing tests still passing
   - Code review approved (using code-review skill)
   - No security issues flagged

---

## Conclusion

**Architecture: ✅ Sound**  
**Risks: Manageable with mitigations**  
**Timeline: 6 days (realistic for solo founder)**  
**Complexity: ~1000 LOC addition (within budget)**

**Recommendation: PROCEED WITH IMPLEMENTATION**

Start with Phase 1 (Foundation) — kind + Argo + MinIO. This validates the infrastructure before writing integration code.

---

*Document generated by Goodra using architecture analysis skills.*
