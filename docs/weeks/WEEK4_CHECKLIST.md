# Week 4 Argo Integration: Quick Checklist

**Reference:** See `WEEK4_ARGO_STRATEGIC_PLAN.md` for detailed analysis.

---

## Pre-Flight Checks

```bash
# Verify local environment
kind version          # Need v0.20+
docker ps             # Docker running
go version            # Need 1.21+
argo version          # Argo CLI installed

# Verify codebase
cd agentic-k8s-operator
go test ./... -v      # All tests passing
```

---

## Implementation Phases

### Phase 1: Foundation (Day 1-2)
- [ ] Create `scripts/setup-kind.sh`
- [ ] Install Argo Workflows v4.0.1 on kind
- [ ] Deploy MinIO to `shared-services` namespace
- [ ] Deploy PostgreSQL for LangGraph checkpointing
- [ ] Verify: `argo list` works, MinIO console accessible

### Phase 2: Containerization (Day 2-3)
- [ ] Create `agents/Dockerfile`
- [ ] Build + load image: `kind load docker-image agents:latest`
- [ ] Deploy Browserless to `shared-services`
- [ ] Deploy LiteLLM proxy to `shared-services`
- [ ] Verify: Agent image runs, Browserless WebSocket works

### Phase 3: WorkflowTemplate (Day 3-4)
- [ ] Create `config/argo/workflowtemplate.yaml`
  - Step 1: Scraper
  - Step 2a/2b: Screenshot + DOM (parallel)
  - Step 3: Suspend gate
  - Step 4: Synthesis
- [ ] Manual test: `argo submit --from workflowtemplate/scraper`
- [ ] Verify: All steps run, suspend works, resume works

### Phase 4: Operator Integration (Day 4-5)
- [ ] Add Argo SDK: `go get github.com/argoproj/argo-workflows/v3`
- [ ] Create `pkg/argo/workflow.go`
  - [ ] `createArgoWorkflow()`
  - [ ] `resumeArgoWorkflow()`
  - [ ] `watchWorkflowStatus()`
- [ ] Create `pkg/argo/workflow_test.go` (unit tests)
- [ ] Update `agentworkload_controller.go` (integration)
- [ ] Update `agentworkload_types.go` (status fields)
- [ ] Run `make manifests`
- [ ] Verify: `go test ./pkg/argo -v` passes

### Phase 5: E2E Validation (Day 5-6)
- [ ] Create `tests/e2e/e2e_test.go`
- [ ] Run full E2E: `make test-e2e`
- [ ] Verify: MinIO contains artifact
- [ ] Code review (use code-review skill)
- [ ] Update documentation

---

## Critical Blockers to Address First

| Blocker | Action |
|---------|--------|
| Python agent Docker image | Create `agents/Dockerfile` |
| PostgreSQL not deployed | Add to `shared-services` |
| Argo SDK missing | `go get github.com/argoproj/argo-workflows/v3@v3.4.0` |

---

## New Files to Create

```
config/argo/workflowtemplate.yaml
config/shared-services/namespace.yaml
config/shared-services/minio.yaml
config/shared-services/browserless.yaml
config/shared-services/litellm.yaml
config/shared-services/postgres.yaml
config/rbac/argo_role.yaml
pkg/argo/workflow.go
pkg/argo/workflow_test.go
agents/Dockerfile
scripts/setup-kind.sh
tests/e2e/e2e_test.go
```

---

## Phase Gate Checks

**After Phase 1:** `argo list` ✓ | MinIO console ✓ | PostgreSQL ✓  
**After Phase 2:** Agent image ✓ | Browserless ✓ | LiteLLM ✓  
**After Phase 3:** Manual workflow ✓ | Suspend/resume ✓  
**After Phase 4:** Unit tests ✓ | Integration tests ✓  
**After Phase 5:** E2E ✓ | Code review ✓ | Docs ✓

---

## No-Go Signals

Stop and reassess if:
- Argo 4.0.1 unstable (fallback: 3.5.x)
- Integration exceeds +500 LOC
- E2E consistently flaky after 3 attempts
- Memory usage > 1GB for workflow controller

---

## Success Criteria

- [ ] AgentWorkload CR → Argo Workflow → MinIO artifact (full pipeline)
- [ ] Suspend gate + OPA approval working
- [ ] All existing tests still passing
- [ ] ~1000 new LOC (within budget)
- [ ] Code review approved
