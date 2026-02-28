# Week 5: Argo Workflows Integration Architecture

**Status:** Phase 1 Brainstorm & Architecture Design  
**Created:** 2026-02-24  
**Target:** Production-grade Argo integration with LangGraph, MinIO, Browserless, LiteLLM  
**Scope:** Days 1-3 (Brainstorm & Design) → Days 4-7 (Implementation)

---

## Executive Summary

Week 5 integrates the Go operator (Weeks 1-4, ~4,000 LOC) with **Argo Workflows** to orchestrate durable, multi-step agent workloads. The architecture is **sound**, leveraging Argo's DAG-based execution, suspend gates, and artifact management while maintaining the operator's generic, tool-agnostic design.

**Key Wins:**
- Operator → Argo Workflow creation (one AgentWorkload CR = one Workflow CR)
- Python agents run as containerized steps with LangGraph checkpointing
- OPA-driven suspend gates for approval workflows
- MinIO artifacts, Browserless web scraping, LiteLLM vision calls
- Durable execution: Pod preemption doesn't lose progress

---

## 1. System Architecture

### 1.1 Full Stack Diagram

```
┌─────────────────────────────────────────────────────────────────────────┐
│ Kubernetes Cluster (k3s, 8-16 GiB)                                     │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│ ┌─ agentic-system namespace ────────────────────────────────────────┐  │
│ │                                                                   │  │
│ │  ┌────────────────────────────────────────────────────────────┐  │  │
│ │  │ Go Operator Reconciler                                    │  │  │
│ │  │ (agentworkload_controller + pkg/argo)                    │  │  │
│ │  ├────────────────────────────────────────────────────────────┤  │  │
│ │  │ 1. Watch AgentWorkload CRs                               │  │  │
│ │  │ 2. OPA evaluate proposed actions                         │  │  │
│ │  │ 3. createArgoWorkflow() → Workflow CR                    │  │  │
│ │  │ 4. Watch Workflow status changes                         │  │  │
│ │  │ 5. Resume on approval (PATCH /resume)                    │  │  │
│ │  │ 6. Update AgentWorkload status                           │  │  │
│ │  └────────────────────────────────────────────────────────────┘  │  │
│ │                           │                                        │  │
│ │                           ├─ creates/watches ────────┐             │  │
│ │                           │                          ▼             │  │
│ │  ┌────────────────────────────────────────────────────────────┐  │  │
│ │  │ Argo Workflows Controller                                 │  │  │
│ │  ├────────────────────────────────────────────────────────────┤  │  │
│ │  │  Executes WorkflowTemplate DAG                           │  │  │
│ │  │                                                           │  │  │
│ │  │  ┌──────────────┐   ┌──────────────────────────────┐    │  │  │
│ │  │  │ Scraper      │──▶│ Parallel (Step 2)           │    │  │  │
│ │  │  │ (Python)     │   ├──────────────────────────────┤    │  │  │
│ │  │  │ (Step 1)     │   │ ├─ Screenshot (Browserless) │    │  │  │
│ │  │  │              │   │ ├─ DOM Extract (jsdom)      │    │  │  │
│ │  │  │ • Fetch HTML │   │ └─ LiteLLM analysis         │    │  │  │
│ │  │  │ • Save raw   │   └──────────────────────────────┘    │  │  │
│ │  │  │   (MinIO)    │                │                      │  │  │
│ │  │  └──────────────┘                ▼                      │  │  │
│ │  │                        ┌──────────────────┐             │  │  │
│ │  │                        │ Suspend Gate     │             │  │  │
│ │  │                        │ (OPA approval)   │             │  │  │
│ │  │                        │ (Step 3)         │             │  │  │
│ │  │                        └──────────────────┘             │  │  │
│ │  │                                │                        │  │  │
│ │  │                        Operator resumed ◄──────────┐    │  │  │
│ │  │                                │                        │  │  │
│ │  │                                ▼                        │  │  │
│ │  │                        ┌──────────────────┐             │  │  │
│ │  │                        │ Synthesis        │             │  │  │
│ │  │                        │ (Python + LLM)   │             │  │  │
│ │  │                        │ (Step 4)         │             │  │  │
│ │  │                        │                  │             │  │  │
│ │  │                        │ • Generate       │             │  │  │
│ │  │                        │   report         │             │  │  │
│ │  │                        │ • Save artifact  │             │  │  │
│ │  │                        │   to MinIO       │             │  │  │
│ │  │                        └──────────────────┘             │  │  │
│ │  │                                │                        │  │  │
│ │  │                                ▼                        │  │  │
│ │  │                        Workflow complete              │  │  │
│ │  └────────────────────────────────────────────────────────┘  │  │
│ │                                                               │  │
│ └───────────────────────────────────────────────────────────────┘  │
│                                                                     │
│ ┌─ shared-services namespace ───────────────────────────────────┐  │
│ │                                                               │  │
│ │  ┌──────────────────────┐  ┌──────────────────────┐          │  │
│ │  │ MinIO                │  │ PostgreSQL           │          │  │
│ │  │ (Artifact storage)   │  │ (LangGraph           │          │  │
│ │  │                      │  │  Checkpointing)      │          │  │
│ │  │ • HTML artifacts     │  │                      │          │  │
│ │  │ • Screenshots        │  │ • Pod preemption     │          │  │
│ │  │ • Reports            │  │   resilience         │          │  │
│ │  │ • JSON metadata      │  │ • State persistence  │          │  │
│ │  └──────────────────────┘  └──────────────────────┘          │  │
│ │                                                               │  │
│ │  ┌──────────────────────┐  ┌──────────────────────┐          │  │
│ │  │ Browserless          │  │ LiteLLM Proxy        │          │  │
│ │  │ (Web screenshots)     │  │ (Vision models)      │          │  │
│ │  │                      │  │                      │          │  │
│ │  │ • Chrome/Puppeteer   │  │ • Routes to GPT-4o   │          │  │
│ │  │ • WebSocket API      │  │   (vision)           │          │  │
│ │  │ • DOM extraction     │  │ • API key mgmt       │          │  │
│ │  └──────────────────────┘  └──────────────────────┘          │  │
│ │                                                               │  │
│ └───────────────────────────────────────────────────────────────┘  │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

### 1.2 Data Flow

```
1. User applies AgentWorkload CR
   └─ Operator reconciles every 30s
   
2. Operator creates Argo Workflow CR
   └─ Sets ownerReference for cascade deletion
   └─ Passes parameters: job_id, target_urls, minio_bucket
   
3. Argo Workflow controller detects new Workflow
   └─ Launches scraper pod (Python agent, Step 1)
   
4. Scraper pod (LangGraph)
   └─ Connects to PostgreSQL checkpoint
   └─ Fetches HTML from target_urls
   └─ Uploads raw_html to MinIO
   └─ Pod may be preempted → checkpointed state persists
   
5. Argo runs Step 2 (parallel): Screenshot + DOM
   └─ Screenshot pod: Browserless → sends to LiteLLM for analysis
   └─ DOM pod: jsdom → extracts structure
   └─ Both upload results to MinIO
   
6. Argo suspends workflow at suspend gate (Step 3)
   └─ Operator polls workflow status
   └─ OPA evaluates synthesized insights
   └─ If confident (>threshold): Operator resumes
   
7. Synthesis step (Step 4)
   └─ LiteLLM vision call on screenshots
   └─ Generate competitive analysis report
   └─ Save report artifact to MinIO
   
8. Workflow completes
   └─ Operator updates AgentWorkload.status.phase = "Completed"
   └─ Operator updates artifact reference for UI
```

---

## 2. Design Decisions & Rationale

### 2.1 Operator-Driven Workflow Creation

**Decision:** Go operator creates Argo Workflow CRs dynamically.

**Rationale:**
- AgentWorkload is source of truth (user-facing CRD)
- Operator handles orchestration logic (approval gates, retries)
- Argo is execution engine (DAG scheduling, pod management)
- Clear separation of concerns

**Trade-offs:**
- ✅ Single CRD for users (familiar pattern)
- ✅ Operator controls workflow lifecycle
- ❌ Adds Argo SDK dependency to Go binary
- ❌ More reconciliation logic (compared to WorkflowTemplate only)

### 2.2 WorkflowTemplate for Reusability

**Decision:** Define WorkflowTemplate once, operator instantiates Workflows from it.

**Rationale:**
- Template captures DAG structure (4 steps, dependencies, parallelism)
- Operator passes parameters: job_id, target_urls, minio_bucket
- Same template runs for all AgentWorkload CRs
- Easier to update DAG structure (edit template, not operator code)

**Template parameters:**
```yaml
parameters:
  - name: job_id
  - name: target_urls
  - name: minio_bucket
  - name: agent_image
  - name: browserless_url
  - name: litellm_url
  - name: postgres_dsn
```

### 2.3 Suspend Gates for OPA Approval

**Decision:** Step 3 is a suspend node, operator resumes after OPA approval.

**Rationale:**
- Argo suspend gates are native (no custom logic needed)
- OPA evaluated in operator (trusted component)
- Human approval possible (future: webhook for manual gates)
- Workflow state persists during suspension

**Flow:**
1. Argo suspends workflow after parallel steps
2. Operator watches workflow status
3. If OPA confident: `argo resume <workflow-name>`
4. Synthesis step continues

### 2.4 LangGraph Checkpointing for Durability

**Decision:** Python agents use LangGraph checkpointing to PostgreSQL.

**Rationale:**
- Pods may be preempted (k3s eviction, resource limits)
- Checkpointing allows resumption from last step
- job_id = thread_id (resume key)
- No state loss on pod restart

**Checkpoint flow:**
```
Pod 1 (Scraper) → Checkpoint: step_1_complete, raw_html={...}
Pod evicted (preemption)
Pod 2 (Scraper resumed) → Read checkpoint → Resume from step 1
  → Run step 2 only
```

### 2.5 MinIO for Artifacts

**Decision:** All intermediate & final artifacts stored in MinIO.

**Rationale:**
- Argo artifact passing requires shared storage
- MinIO provides S3-compatible API (cost-effective)
- Artifacts persist beyond workflow lifetime
- Easy to inspect for debugging

**Artifacts:**
- Step 1: `s3://<bucket>/job_<id>/raw_html.json`
- Step 2: `s3://<bucket>/job_<id>/screenshots/*.png`
- Step 2: `s3://<bucket>/job_<id>/dom_structures.json`
- Step 4: `s3://<bucket>/job_<id>/report.md`

---

## 3. Integration Points

### 3.1 Operator ↔ Argo Workflows

**New code in `pkg/argo/workflow.go`:**

```go
// Create workflow from template
func (r *AgentWorkloadReconciler) createArgoWorkflow(
    ctx context.Context,
    workload *agenticv1alpha1.AgentWorkload,
) (*unstructured.Unstructured, error)

// Watch workflow status
func (r *AgentWorkloadReconciler) watchArgoWorkflowStatus(
    ctx context.Context,
    workload *agenticv1alpha1.AgentWorkload,
) (string, error)

// Resume suspended workflow
func (r *AgentWorkloadReconciler) resumeArgoWorkflow(
    ctx context.Context,
    workflowName string,
) error
```

**Updates to `agentworkload_controller.go`:**
- Call `createArgoWorkflow()` after OPA initial approval
- Poll workflow status (or use watch event)
- Call `resumeArgoWorkflow()` after suspension gate evaluation
- Handle workflow failures (retry, timeout)

### 3.2 Argo Workflows ↔ Python Agents

**Environment variables passed to pod:**
```bash
JOB_ID=job_12345
TARGET_URLS=["https://example.com"]
MINIO_BUCKET=artifacts
MINIO_URL=http://minio.shared-services:9000
BROWSERLESS_URL=ws://browserless.shared-services:3000
LITELLM_URL=http://litellm.shared-services:8000
POSTGRES_DSN=postgresql://user:pass@postgres:5432/langgraph
```

**Pod mounts:**
- Secret: MinIO credentials (`/etc/secrets/minio-keys`)
- Secret: LiteLLM API key (`/etc/secrets/llm-keys`)
- ConfigMap: LangGraph model config (`/etc/config/graph`)

### 3.3 Python Agents ↔ Dependencies

**Browserless (WebSocket):**
```python
# agents/tools/browserless.py
websocket.connect("ws://browserless:3000")
screenshot = ws.send({"url": target_url})
dom = ws.send({"url": target_url, "action": "getDOM"})
```

**LiteLLM (HTTP REST):**
```python
# agents/tools/litellm_client.py
response = litellm.completion(
    model="gpt-4o",
    messages=[{"role": "user", "content": image_data}]
)
```

**PostgreSQL (Checkpointing):**
```python
# agents/graph/workflow.py
from langgraph.checkpoint.postgres import PostgresSaver
checkpointer = PostgresSaver.from_conn_string(dsn)
config = {"configurable": {"thread_id": job_id}}
result = workflow.invoke(initial_state, config=config)
```

---

## 4. Dependency & Risk Management

### 4.1 Critical Dependencies

| Component | Version | Risk | Mitigation |
|-----------|---------|------|-----------|
| Argo Workflows | v4.0.1 | API changes | Pin version, test on kind |
| LangGraph | v0.2+ | Checkpoint schema | Use latest stable release |
| Browserless | v1.6+ | WebSocket timeout | Add retry logic, circuit breaker |
| LiteLLM | v1.5+ | Proxy rate limit | Cache responses, backoff |
| PostgreSQL | 14+ | Connection pool | Configure pool size, test load |
| MinIO | latest | S3 compatibility | Use moto for testing |

### 4.2 Key Risks & Mitigations

| Risk | Severity | Mitigation |
|------|----------|-----------|
| **Workflow CR ownership** | HIGH | Set `ownerReference` → cascade delete when AgentWorkload deleted |
| **Suspend gate deadlock** | MEDIUM | Add 30m timeout, auto-deny if not approved |
| **Pod preemption loses state** | MEDIUM | LangGraph checkpointing to PostgreSQL (mandatory) |
| **Argo controller crash** | MEDIUM | AgentWorkload times out after 5m, marks failed |
| **MinIO unavailable** | MEDIUM | Retry 3x with exponential backoff, fail gracefully |
| **LiteLLM rate limit** | LOW | Implement token bucket, cache responses |
| **Browserless OOMKilled** | MEDIUM | Set memory limit (2GB), use headless mode |
| **PostgreSQL connection exhaustion** | LOW | Connection pool limit (10), timeout 30s |

### 4.3 Network & Security

**RBAC for Argo:**
```yaml
apiGroups: ["argoproj.io"]
resources: ["workflows", "workflowtemplates"]
verbs: ["get", "list", "watch", "create", "update", "patch"]
```

**Pod Security:**
- Non-root user (UID 1000)
- Read-only filesystem root
- No privileged mode
- Network policies: agents → MinIO/Browserless/LiteLLM only

**Secrets Management:**
- MinIO credentials: Kubernetes Secret
- LiteLLM API key: Kubernetes Secret
- PostgreSQL password: Kubernetes Secret
- Mounted at `/etc/secrets/` (read-only)

---

## 5. Testing Strategy

### 5.1 Test Pyramid

```
            E2E (1 test, 15min)
           ┌─────────────┐
          │  Full pipeline   │
          │  CI/CD gate      │
          └────────┬────────┘
                   │
        Integration (5-10 tests, 5min)
           ┌─────────────┐
          │  Operator-Argo   │
          │  Operator-Agent  │
          │  Workflow status │
          └────────┬────────┘
                   │
         Unit (20+ tests, 30sec)
           ┌─────────────┐
          │  Workflow build  │
          │  Parameter pass  │
          │  Resume logic    │
          └─────────────┘
```

### 5.2 Unit Tests (`pkg/argo/workflow_test.go`)

```go
TestBuildWorkflowFromTemplate
  ✓ Workflow YAML generated from parameters
  ✓ ownerReference set correctly
  ✓ Parameters passed (job_id, target_urls, minio_bucket)

TestResumeWorkflowRequest
  ✓ PATCH /workflows/{namespace}/{name}/resume payload

TestWorkflowStatusParsing
  ✓ Extract status phase from Workflow CR
  ✓ Detect suspend node vs completion
```

### 5.3 Integration Tests (`internal/controller/*_test.go`)

```go
TestReconciler_CreatesArgoWorkflow
  ✓ AgentWorkload applied → Workflow CR created
  ✓ Workflow inherits AgentWorkload name/namespace

TestReconciler_WatchesWorkflowStatus
  ✓ Workflow completes → AgentWorkload status updated

TestReconciler_ResumesOnApproval
  ✓ Workflow suspended → Operator calls resume
  ✓ Synthesis step continues

TestReconciler_FailsOnWorkflowTimeout
  ✓ Workflow pending >5m → AgentWorkload marked failed
```

### 5.4 E2E Test (`tests/e2e/e2e_test.go`)

**Scenario:** Full competitor scraping pipeline

```gherkin
Given:
  - kind cluster with Argo v4.0.1
  - MinIO, PostgreSQL, Browserless, LiteLLM deployed
  - WorkflowTemplate applied

When:
  - Apply AgentWorkload CR (target_urls: ["https://example.com"])

Then:
  - Argo Workflow CR created
  - Scraper step completes (HTML artifact in MinIO)
  - Parallel steps complete (Screenshot + DOM)
  - Workflow suspends at gate
  - Operator resumes (confidence > threshold)
  - Synthesis step completes (report in MinIO)
  - AgentWorkload.status.phase = "Completed"
```

**Test environment:** `kind` with extra port mappings for Argo UI, MinIO console

---

## 6. Implementation Order

### Phase 1: Foundation (Days 1-2)

1. **1A. Setup kind + Argo**
   - Create `scripts/setup-kind.sh`
   - Install Argo Workflows v4.0.1
   - Verify Argo controller running

2. **1B. Deploy shared services**
   - PostgreSQL (LangGraph checkpoints)
   - MinIO (artifact storage)
   - Browserless (web scraping)
   - LiteLLM proxy (vision models)

### Phase 2: WorkflowTemplate Design (Days 2-3)

3. **2A. Create WorkflowTemplate**
   - 4 steps: scraper → parallel (screenshot+dom) → suspend → synthesis
   - Parameters: job_id, target_urls, minio_bucket, etc.
   - Artifact passing via MinIO

4. **2B. Test template manually**
   - `argo submit --from workflowtemplate/scraper`
   - Verify each step succeeds
   - Test suspend & resume

### Phase 3: Operator Integration (Days 3-5)

5. **3A. Add Argo SDK to go.mod**
   - `go get github.com/argoproj/argo-workflows/v3`
   - Create `pkg/argo/` package

6. **3B. Implement workflow creation**
   - `createArgoWorkflow()` in `pkg/argo/workflow.go`
   - Unit tests (3-5 tests)

7. **3C. Update controller reconciliation**
   - Call `createArgoWorkflow()` after OPA approval
   - Watch workflow status (poll or events)
   - Call `resumeArgoWorkflow()` after suspend gate

8. **3D. Update CRD types**
   - Add `ArgoWorkflowRef` to status
   - Add `ArgoPhase` to status
   - `make manifests`

### Phase 4: E2E Validation (Days 5-7)

9. **4A. Write E2E tests**
   - `tests/e2e/e2e_test.go`
   - Full pipeline verification

10. **4B. Run & debug**
    - `make test-e2e`
    - Verify MinIO artifact exists
    - Verify AgentWorkload.status.phase = "Completed"

11. **4C. Code review & documentation**
    - Review all new code
    - Update WEEK5_SUMMARY.md
    - Tag commit

---

## 7. Success Criteria

### Functional Requirements
- [ ] AgentWorkload → Argo Workflow creation works
- [ ] All 4 DAG steps execute successfully
- [ ] Suspend gate pauses workflow
- [ ] Operator resumes on OPA approval
- [ ] Artifacts stored in MinIO
- [ ] Pod preemption doesn't lose progress (LangGraph checkpoint)

### Code Quality
- [ ] All unit tests passing (20+ tests)
- [ ] All integration tests passing (5-10 tests)
- [ ] E2E test passing
- [ ] Code review approved
- [ ] No security issues (SSRF, secrets, RBAC)

### Documentation
- [ ] WEEK5_SUMMARY.md with implementation details
- [ ] Inline code comments for new functions
- [ ] README updated with Argo section

---

## 8. Files to Create/Modify

### New Files
| File | Purpose | LOC |
|------|---------|-----|
| `pkg/argo/workflow.go` | Workflow creation & resume | 200 |
| `pkg/argo/workflow_test.go` | Unit tests | 150 |
| `config/argo/workflowtemplate.yaml` | DAG definition | 100 |
| `config/shared-services/namespace.yaml` | Namespace | 5 |
| `config/shared-services/minio.yaml` | MinIO deployment | 50 |
| `config/shared-services/postgres.yaml` | PostgreSQL deployment | 50 |
| `config/shared-services/browserless.yaml` | Browserless deployment | 50 |
| `config/shared-services/litellm.yaml` | LiteLLM deployment | 50 |
| `scripts/setup-kind.sh` | kind + Argo setup | 80 |
| `tests/e2e/e2e_test.go` | E2E tests | 150 |
| `config/rbac/argo_role.yaml` | RBAC for Argo | 15 |

**Total new LOC: ~900**

### Modified Files
| File | Change | LOC |
|------|--------|-----|
| `internal/controller/agentworkload_controller.go` | Argo workflow integration | +100 |
| `api/v1alpha1/agentworkload_types.go` | Add ArgoWorkflowRef, ArgoPhase | +20 |
| `agents/entrypoint.py` | MinIO upload on completion | +30 |
| `go.mod` | Add Argo SDK | +5 |

**Total modified LOC: ~155**

---

## 9. Key Metrics

| Metric | Target |
|--------|--------|
| **Code addition** | <1,000 LOC (within budget) |
| **Test coverage** | 90%+ (new code) |
| **E2E test time** | <20 minutes |
| **Binary size increase** | <50MB (Argo SDK) |
| **Controller memory** | <200MB (w/ Argo client) |

---

## Conclusion

**Architecture Status:** ✅ **SOUND & READY**

Week 5 builds a production-grade integration of:
- Go operator (orchestration, approval gates, policy)
- Argo Workflows (DAG execution, suspend gates, artifacts)
- Python agents (LangGraph, checkpointing, tool calling)
- External services (MinIO, Browserless, LiteLLM, PostgreSQL)

**Recommended start date:** Immediately after WEEK 4 completion  
**Expected completion:** 7 days  
**Go/No-Go:** **PROCEED**

---

*Document created by subagent w5-phase1-brainstorm (2026-02-24)*  
*Next: Implementation phase begins upon requester approval*
