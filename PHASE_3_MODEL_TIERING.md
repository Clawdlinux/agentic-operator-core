# Phase 3: Model Tiering for Cost Optimization

**Objective:** Reduce inference costs by 60-80% through intelligent model routing

## Architecture

### Current State
- All agents use Claude (fixed, expensive)
- No task-specific routing
- Missing cost attribution

### Target State
- **Task Classification Layer** — Analyze each task complexity
- **Cost-Aware Routing** — Route based on requirements:
  - **Phi-3.5** (cheapest, 0.01/1K tokens) — Simple validation, formatting, parsing
  - **Mixtral** (medium, 0.27/1K tokens) — Analysis, synthesis, multi-step reasoning  
  - **Claude** (expensive, 0.735/1K tokens) — Complex reasoning, novel problems, consensus voting
- **Cost Attribution** — Track per-task model selection + savings

## Implementation Plan

### Step 1: Task Classifier (Days 1-2)
```go
type TaskCategory string

const (
    CategoryValidation TaskCategory = "validation"    // Phi-3.5
    CategoryAnalysis   TaskCategory = "analysis"      // Mixtral
    CategoryReasoning  TaskCategory = "reasoning"     // Claude
)

func ClassifyTask(prompt string) TaskCategory {
    // Analyze prompt:
    // - Length (< 200 chars → validation)
    // - Keywords (verify, check, parse → validation)
    // - Complexity (analyze, synthesize → analysis)
    // - Novel/uncertain → reasoning
}
```

**Tests:**
- ✅ Classify validation prompts → Phi-3.5
- ✅ Classify analysis prompts → Mixtral
- ✅ Classify reasoning prompts → Claude
- ✅ Edge cases (ambiguous prompts → default to Claude)

### Step 2: LiteLLM Integration (Days 2-3)
Activate existing LiteLLM integration with model routing:
```yaml
# litellm config
models:
  - model_name: phi-3.5
    litellm_params:
      model: "ollama/phi-3.5"
      api_base: "http://litellm:4000"
    cost: {input: 0.00001, output: 0.00001}
    
  - model_name: mixtral
    litellm_params:
      model: "ollama/mixtral"
      api_base: "http://litellm:4000"
    cost: {input: 0.00027, output: 0.00027}
    
  - model_name: claude
    litellm_params:
      model: "anthropic/claude-3.5-sonnet"
    cost: {input: 0.003, output: 0.015}
```

**Tests:**
- ✅ Route validation → Phi-3.5 via LiteLLM
- ✅ Route analysis → Mixtral via LiteLLM
- ✅ Route reasoning → Claude via LiteLLM
- ✅ Cost tracking per request

### Step 3: Operator Integration (Days 3-4)
Add to `AgentWorkload` CRD:
```yaml
apiVersion: agentic.io/v1
kind: AgentWorkload
metadata:
  name: market-intelligence
spec:
  agent:
    modelStrategy: cost-aware  # NEW
    taskClassifier: default     # NEW
  instructions: "Analyze market sentiment for AAPL..."
```

Controller logic:
```go
if spec.ModelStrategy == "cost-aware" {
    taskCategory := classifier.Classify(spec.Instructions)
    modelName := categoryToModel[taskCategory]
    // Set in operator reconciliation
}
```

**Tests:**
- ✅ AgentWorkload with modelStrategy: cost-aware
- ✅ Task classifier applied during reconciliation
- ✅ Correct model selected based on task type

### Step 4: Cost Attribution (Days 4-5)
Add metrics:
```go
// prometheus metrics
modelRoutingCounter = prometheus.NewCounterVec(
    prometheus.CounterOpts{Name: "agentic_model_routing_total"},
    []string{"task_category", "model_name"},
)

estimatedSavingsGauge = prometheus.NewGaugeVec(
    prometheus.GaugeOpts{Name: "agentic_estimated_savings_usd"},
    []string{"period"}, // "daily", "weekly", "monthly"
)
```

**Grafana Dashboard:**
- Model distribution (pie chart: Phi vs Mixtral vs Claude)
- Cost per category (bar chart)
- Estimated monthly savings vs current run

**Tests:**
- ✅ Metrics emitted correctly
- ✅ Cost calculations accurate
- ✅ Savings estimates reasonable

### Step 5: Trajectory Logging (Days 5)
Enable OpenTelemetry for agent trajectory:
```go
tracer.Start(ctx, "agent_execution")
  tracer.Start(ctx, "task_classification") // → Phi-3.5
  tracer.Start(ctx, "model_invocation")
  tracer.Start(ctx, "result_validation")  // → Phi-3.5
tracer.End()
```

**Queries:**
```
SELECT span.name, model, duration_ms
FROM agent_traces
WHERE workload_name = "market-intelligence"
ORDER BY timestamp DESC
```

## Testing Strategy

### Unit Tests (pkg/routing/)
```go
func TestClassifyValidation(t *testing.T) { ... }
func TestClassifyAnalysis(t *testing.T) { ... }
func TestClassifyReasoning(t *testing.T) { ... }
func TestLiteLLMRouting(t *testing.T) { ... }
func TestCostCalculation(t *testing.T) { ... }
```

### Integration Tests (integration/)
```go
func TestAgentWorkloadWithCostAware(t *testing.T) {
    // Deploy AgentWorkload with modelStrategy: cost-aware
    // Verify task classifier runs
    // Verify correct model used
    // Verify cost metrics emitted
}
```

### E2E Tests (e2e/)
```bash
# Deploy full operator with model tiering
kubectl apply -f test-workloads/cost-aware-agent.yaml

# Monitor for 5 minutes
# Verify:
# - Phi-3.5 used for validation tasks
# - Mixtral used for analysis tasks
# - Claude used only for reasoning
# - Cost savings in Grafana dashboard
```

## Expected Outcomes

**Cost Reduction:**
- Baseline (all Claude): $1,000/month
- With Model Tiering: $200-300/month
- **Savings: 70-80%**

**Quality Maintenance:**
- Task validation still accurate (Phi-3.5 sufficient)
- Analysis quality maintained (Mixtral adequate)
- Complex reasoning still uses Claude (preserves quality)
- Consensus voting still uses Claude (decision quality critical)

**Metrics:**
- `agentic_model_routing_total` by category
- `agentic_estimated_savings_usd` monthly
- Model distribution dashboard
- Trajectory traces for debugging

## Timeline

| Day | Task | Deliverable |
|-----|------|-------------|
| 1-2 | Task Classifier | `pkg/routing/classifier.go` + unit tests |
| 2-3 | LiteLLM Integration | Config + integration tests |
| 3-4 | Operator CRD | AgentWorkload spec + controller logic |
| 4-5 | Cost Attribution | Prometheus metrics + Grafana dashboard |
| 5 | Trajectory Logging | OpenTelemetry integration |
| 5 | E2E Testing | Full cluster validation |

**Total: 5 days (10-15 dev hours)**

## Success Criteria

✅ Model tiering implemented and tested
✅ E2E tests passing (validation, analysis, reasoning tasks)
✅ Cost savings measurable (70-80% reduction)
✅ Quality maintained (no accuracy regression)
✅ Dashboard shows real-time model distribution + savings
✅ Trajectory logs enable debugging and optimization
✅ Ready for Phase 4 (agent evaluation pipeline)

---

**Next:** Spawn coding agents to implement model tiering framework.
