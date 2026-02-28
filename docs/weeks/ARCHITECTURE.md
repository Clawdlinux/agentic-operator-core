# Architecture - Agentic Kubernetes Operator

**This document describes the architecture of the Generic AgentWorkload Operator.**

See also: `WEEK1_SUMMARY.md` for detailed design decisions and implementation notes.

---

## High-Level Overview

The operator follows a simple, tool-agnostic pattern:

```
AgentWorkload CR 
    ↓
Operator Reconciler
    ├─ Fetch AgentWorkload
    ├─ Call MCP: get_status()
    ├─ Call MCP: propose_action()
    ├─ Update AgentWorkload.status
    └─ Requeue after 30s
    ↓
MCP Server (ANY tool: Ceph, MinIO, PostgreSQL, etc.)
```

**Key:** No hardcoded infrastructure-specific logic. The MCP endpoint tells the operator what tool to talk to.

---

## Components

### 1. AgentWorkload CRD

**Location:** `api/v1alpha1/agentworkload_types.go`

**Spec Fields:**
- `workloadType`: enum (generic, ceph, minio, postgres, aws, kubernetes)
- `mcpServerEndpoint`: HTTP/HTTPS URL to MCP server
- `objective`: string describing what the agent should do
- `agents`: list of agent names
- `autoApproveThreshold`: confidence level for auto-approval
- `opaPolicy`: safety policy (strict/permissive)

**Status Fields:**
- `phase`: Pending, Running, Completed, Failed
- `readyAgents`: number of agents ready
- `lastReconcileTime`: when reconciliation last ran
- `proposedActions`: what agents suggest
- `executedActions`: what was approved and executed
- `conditions`: K8s Conditions (Ready, Progressing, Degraded)

### 2. MCP Client

**Location:** `pkg/mcp/client.go`

Generic HTTP client for ANY MCP server:

```go
client := mcp.NewMCPClient("http://mcp-server:8000")

// List available tools
tools, err := client.ListTools()

// Call any tool
result, err := client.CallTool("get_status", map[string]interface{}{})
```

**Design:** Zero infrastructure-specific code. Works with Ceph, MinIO, PostgreSQL, AWS, Kubernetes, or any other MCP server.

### 3. Mock MCP Server

**Location:** `pkg/mcp/mock_server.go`

Fake MCP server for testing:
```go
mockServer := mcp.NewMockServer(":9001")
go mockServer.Start()
defer mockServer.Stop()
```

Implements:
- `GET /tools` → Returns list of available tools
- `POST /call_tool` → Returns mocked responses

**Benefit:** Full testing without real infrastructure.

### 4. Reconciliation Loop

**Location:** `internal/controller/agentworkload_controller.go`

State machine:
1. Watch AgentWorkload CR
2. Call MCP `get_status()` → understand current state
3. Call MCP `propose_action()` → get recommendation
4. Store proposed action in status
5. Update `status.phase = "Running"`
6. Requeue after 30 seconds

**Design:** Idempotent, safe for Kubernetes. Crashes/evictions don't lose state.

---

## Design Decisions

### 1. Tool-Agnostic Architecture
**Choice:** Single CRD and controller for all infrastructure types.

**Why:** Write once, deploy everywhere. Same code works for Ceph, MinIO, PostgreSQL, AWS, Kubernetes.

**Trade-off:** Less special-casing per tool = more generic = flexible.

### 2. HTTP-Based MCP Bridge
**Choice:** Call MCP servers over HTTP (not gRPC, not custom protocol).

**Why:** Simple, language-independent, works with any MCP implementation.

**Trade-off:** HTTP overhead vs protocol elegance (negligible for 30s requeue interval).

### 3. Mock Server for Testing
**Choice:** Full test suite WITHOUT real infrastructure.

**Why:** Faster feedback loop, 100% test coverage, zero infrastructure cost during development.

**Trade-off:** Mock responses might not match real tool behavior (caught in Phase 2 integration tests).

### 4. Idempotent Reconciliation
**Choice:** 30-second requeue loop, not event-driven.

**Why:** Simple, safe, works with K8s pod lifecycle (crashes, evictions, preemption).

**Trade-off:** Slower response time than event-driven (30s vs <100ms). Good tradeoff for Phase 1.

### 5. Kubernetes Native Patterns
**Choice:** Follow K8s API conventions (Conditions, Phases, Finalizers).

**Why:** Works with kubectl, integrates with ecosystem, familiar to operators.

**Trade-off:** More boilerplate than custom patterns (worth it for maintainability).

---

## Kubernetes Manifests

### CRD Manifest
Auto-generated from `agentworkload_types.go`:
```yaml
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: agentworkloads.agentic.ninerewards.io
spec:
  group: agentic.ninerewards.io
  names:
    kind: AgentWorkload
    plural: agentworkloads
  scope: Namespaced
  versions:
  - name: v1alpha1
    served: true
    storage: true
    schema:
      openAPIV3Schema:
        # ... see config/crd/bases/ for full schema
```

### Example AgentWorkload
```yaml
apiVersion: agentic.ninerewards.io/v1alpha1
kind: AgentWorkload
metadata:
  name: generic-optimization-task
  namespace: default
spec:
  workloadType: generic
  mcpServerEndpoint: "http://mcp-server:8000"
  objective: "Optimize system performance"
  agents: ["analyzer", "optimizer", "monitor"]
  autoApproveThreshold: "0.95"
  opaPolicy: strict
```

---

## Deployment

### Build
```bash
make manifests   # Generate CRD manifests
go build -o bin/manager ./cmd/main.go
```

### Deploy
```bash
kubectl apply -f config/crd/bases/agentworkload_crd.yaml
kubectl apply -f bin/manager
```

### Verify
```bash
kubectl get crd agentworkloads.agentic.ninerewards.io
kubectl get agentworkload -A
```

---

## Testing

### Unit Tests
```bash
go test ./pkg/mcp -v
# 6/6 tests passing, 100% coverage
```

### Integration Tests (Phase 2)
- Deploy operator to k3s
- Create sample AgentWorkload
- Verify reconciliation runs
- Verify webhook validation

---

## Future Phases

### Phase 2: Webhook Validation + OPA
- Validating webhook (reject invalid specs)
- OPA policy engine (safety policies)
- Action execution (approve/reject based on confidence)

### Phase 3: Agent Integration
- Python agent bridge
- LangGraph with checkpointing
- Multi-agent coordination

### Phase 4: Production
- Helm chart
- CI/CD pipeline
- Documentation
- v0.1.0 release

---

**See WEEK1_SUMMARY.md for detailed design documentation.**
