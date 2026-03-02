# Phase 8: Auto-Scaling Based on SLA

**Objective:** Automatically scale LLM provider resources when SLA metrics degrade, with intelligent model selection to optimize cost and performance.

---

## Architecture

```
┌──────────────────────────────────────────────┐
│ SLA Monitoring Loop (Every 30s)              │
├──────────────────────────────────────────────┤
│                                              │
│  1. Read SLA metrics from all tenants        │
│  2. Calculate success rate trend             │
│  3. Detect if approaching breach             │
│  4. Trigger auto-scaling actions:            │
│     ├─ Scale up LLM replicas (+1)            │
│     ├─ Downgrade model tier (save cost)      │
│     └─ Retry with fallback provider          │
│  5. Record scaling event (audit trail)       │
│                                              │
└──────────────────────────────────────────────┘
```

## Components

### 1. ScalingPolicy (auto-scaling rules)

```go
type ScalingPolicy struct {
    Name                 string
    TenantName           string
    SLATarget            float64    // e.g., 99.0
    LowerBound           float64    // e.g., 97.0 (trigger scale-up)
    UpperBound           float64    // e.g., 99.5 (trigger scale-down)
    
    ScaleUpThreshold     float64    // % below target (e.g., 2% below = scale up)
    ScaleDownThreshold   float64    // % above target (e.g., 0.5% above = scale down)
    
    MinReplicas          int        // e.g., 1
    MaxReplicas          int        // e.g., 10
    CooldownSeconds      int        // Wait before next scale action
    LastScaledAt         time.Time
}
```

### 2. ModelSelector (cost-aware downgrade)

```go
type ModelTier string

const (
    TierCheap    ModelTier = "cheap"    // gpt-3.5, llama-2-7b
    TierMedium   ModelTier = "medium"   // gpt-4, llama-2-70b
    TierExpensive ModelTier = "expensive" // gpt-4-turbo, claude-opus
)

type ModelSelector struct {
    CurrentTier     ModelTier
    AvailableTiers  []ModelTier
    CostPerToken    map[ModelTier]float64
}

// AutoDowngrade returns the next cheaper tier
func (ms *ModelSelector) AutoDowngrade() (ModelTier, error)

// AutoUpgrade returns the next more powerful tier
func (ms *ModelSelector) AutoUpgrade() (ModelTier, error)
```

### 3. ScalingController (orchestrates scaling)

```go
type ScalingController struct {
    slaMonitor *multitenancy.SLAMonitor
    k8sClient  *kubernetes.Clientset
    policies   map[string]*ScalingPolicy
}

func (sc *ScalingController) Reconcile(ctx context.Context, tenantName string) error {
    // 1. Get current SLA status
    slaStatus, _ := sc.slaMonitor.GetStatus(tenantName)
    
    // 2. Get policy for this tenant
    policy, ok := sc.policies[tenantName]
    if !ok { return nil }
    
    // 3. Check if scaling needed
    if slaStatus.SuccessRatePercent < policy.LowerBound {
        return sc.scaleUp(ctx, tenantName)
    }
    if slaStatus.SuccessRatePercent > policy.UpperBound {
        return sc.scaleDown(ctx, tenantName)
    }
    return nil
}

func (sc *ScalingController) scaleUp(ctx context.Context, tenantName string) error {
    // 1. Check cooldown
    policy := sc.policies[tenantName]
    if time.Since(policy.LastScaledAt) < time.Duration(policy.CooldownSeconds)*time.Second {
        return nil // Still in cooldown
    }
    
    // 2. Increase LLM provider replicas
    // kubectl patch deployment -n <namespace> <provider> -p '{"spec":{"replicas": N+1}}'
    
    // 3. Update policy
    policy.LastScaledAt = time.Now()
    
    // 4. Audit log
    return nil
}

func (sc *ScalingController) scaleDown(ctx context.Context, tenantName string) error {
    // Reverse of scaleUp (decrease replicas)
    return nil
}
```

## Tests

```go
// scaling_policy_test.go
TestScalingPolicyCheckThreshold()      // Detect when to scale
TestAutoDowngradeModelTier()           // Cost optimization
TestAutoUpgradeModelTier()             // Performance boost
TestCooldownEnforcement()              // Prevent flapping

// model_selector_test.go
TestModelSelectorDowngrade()           // Cheap→Medium→Expensive
TestModelSelectorCostEstimate()        // Calculate savings
TestFallbackChain()                    // If model fails, try next

// scaling_controller_test.go
TestScalingControllerScaleUp()         // Replicas increase
TestScalingControllerScaleDown()       // Replicas decrease
TestScalingControllerRespectsCooldown()// No rapid scaling
```

## Deployment

```bash
# 1. Define scaling policies per tenant
kubectl create configmap scaling-policies \
  -n agentic-system \
  --from-literal=customer-alpha='{"slaTarget":99.0,"scaleUpThreshold":2.0,"minReplicas":1,"maxReplicas":5}'

# 2. Deploy scaling controller as sidecar or separate reconciler
# (runs in agentic-system namespace, watches all tenants)

# 3. Monitor scaling events
kubectl logs -n agentic-system -l app=scaling-controller --tail=50

# 4. Check LLM provider scaling
kubectl get deployment -n shared-services llm-* -o wide
```

## Success Criteria

✅ Detect SLA degradation (success rate < target)
✅ Scale LLM provider replicas dynamically
✅ Downgrade model tier to save cost
✅ Enforce cooldown (prevent flapping)
✅ Audit trail (record all scaling events)
✅ All tests passing
✅ No manual intervention needed

---

**Status:** Ready to implement 🚀
