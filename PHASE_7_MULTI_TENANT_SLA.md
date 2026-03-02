# Phase 7: Multi-Tenant & SLA Enforcement

**Objective:** Enable multiple customers to run on shared agentic-prod cluster with isolated quotas, metrics, and SLA guarantees.

**Timeline:** 1-2 days | **Deployment:** agentic-prod cluster

---

## Vision

Transform from single-tenant to enterprise multi-tenant architecture:

```
┌─────────────────────────────────────────┐
│ Shared Kubernetes Cluster               │
├─────────────────────────────────────────┤
│                                         │
│  Customer A Namespace      Customer B   │
│  ├─ Quota: 10 seats        Namespace    │
│  ├─ SLA: 99.9%             ├─ Quota: 5  │
│  ├─ Cost budget: $1000/mo  │   seats    │
│  └─ Workloads: 100-200     ├─ SLA: 99%  │
│                             └─ Cost: $250/mo
│                                         │
│  Shared Services:                       │
│  ├─ PostgreSQL (per-customer DBs)       │
│  ├─ LLM Providers (metered)             │
│  ├─ Observability (per-customer views)  │
│  └─ License Enforcement (per-customer)  │
│                                         │
└─────────────────────────────────────────┘
```

---

## Architecture

### 1. Namespace Isolation

**File:** `pkg/multitenancy/isolation.go`

```go
type TenantContext struct {
	Name              string          // customer-alpha
	Namespace         string          // agentic-customer-alpha
	License           *License        // Seat count, tier, features
	QuotaPerDay       int64           // Max tasks per day
	ResourceQuota     ResourceLimits  // CPU, memory, storage
	CostBudgetUSD     float64
	SLATarget         float64         // 99.0 for 99%, 99.9 for 99.9%, etc.
}

// Extracted from request headers, JWT claims, or namespace annotations
func ExtractTenant(ctx context.Context, namespace string) (*TenantContext, error)
```

### 2. Request-Level Quotas

**File:** `pkg/multitenancy/quota.go`

```go
type QuotaManager struct {
	mu      sync.RWMutex
	tenants map[string]*TenantQuota
}

type TenantQuota struct {
	Name            string
	TasksPerDay     int
	TasksRemaining  int // Decremented per workload
	LastReset       time.Time
	CurrentCostUSD  float64
	CostBudgetUSD   float64
	IsActive        bool
}

// Check before reconciliation
func (qm *QuotaManager) CheckQuota(ctx context.Context, tenant *TenantContext) error {
	// Return ErrQuotaExceeded if tenant has no remaining tasks
}

// Decrement on workload creation
func (qm *QuotaManager) ConsumeQuota(tenantName string, tasks int) error
```

### 3. SLA Monitoring & Enforcement

**File:** `pkg/multitenancy/sla.go`

```go
type SLAMonitor struct {
	mu       sync.RWMutex
	trackers map[string]*SLATracker
}

type SLATracker struct {
	TenantName       string
	SuccessCount     int
	FailureCount     int
	SuccessRate      float64 // % (50-100)
	SLATarget        float64 // e.g., 99.0 for 99%
	IsBreached       bool
	LastBreach       *time.Time
}

// Track every workload result
func (sm *SLAMonitor) RecordResult(tenantName string, success bool)

// Get current SLA status
func (sm *SLAMonitor) GetStatus(tenantName string) *SLATracker
```

### 4. Multi-Customer Views in Grafana

**File:** `config/grafana/tenant-dashboard.json`

```json
{
  "title": "Tenant View — [[tenant]]",
  "panels": [
    {
      "title": "Task Quota Usage",
      "targets": [{
        "expr": "agentic_workload_count{tenant=\"[[tenant]]\"}"
      }]
    },
    {
      "title": "SLA Status",
      "targets": [{
        "expr": "agentic_workload_success_rate{tenant=\"[[tenant]]\"}"
      }]
    },
    {
      "title": "Cost This Month",
      "targets": [{
        "expr": "agentic_workload_cost_usd_total{tenant=\"[[tenant]]\"}"
      }]
    }
  ]
}
```

### 5. Per-Tenant License Enforcement

**Extension to Phase 1 License System**

```go
// In license/validator.go
func (v *Validator) EnforcePerTenant(tenantName string, license *License, workloadCount int) error {
	// Check: workloadCount <= license.MaxSeats
	// Return error if exceeded
}

// In controller
if err := r.LicenseValidator.EnforcePerTenant(tenant.Name, tenant.License, workloadCount); err != nil {
	// Reject workload creation
	return errors.New("license seat limit exceeded for tenant")
}
```

---

## Implementation Plan

### Step 1: Multitenancy Types (Day 1 AM)

```bash
# Create types package
mkdir -p pkg/multitenancy
touch pkg/multitenancy/types.go       # TenantContext, License structs
touch pkg/multitenancy/isolation.go   # ExtractTenant() implementation
```

### Step 2: Quota Manager (Day 1 PM)

```bash
touch pkg/multitenancy/quota.go       # QuotaManager + quota tracking
touch pkg/multitenancy/quota_test.go  # 10+ quota tests
```

### Step 3: SLA Monitor (Day 2 AM)

```bash
touch pkg/multitenancy/sla.go         # SLAMonitor + breach detection
touch pkg/multitenancy/sla_test.go    # 8+ SLA tests
```

### Step 4: Controller Integration (Day 2 PM)

```bash
# Update controller
# - Extract tenant from request context
# - Check quota before reconciliation
# - Track SLA on completion
# - Reject if quota exceeded or SLA at risk
```

### Step 5: Observability (Day 2 Late)

```bash
touch pkg/multitenancy/metrics.go     # Tenant-scoped Prometheus metrics
# Update Grafana dashboards
# Create per-tenant views
```

---

## Tests

| File | Count | Key Tests |
|------|-------|-----------|
| isolation_test.go | 5 | Extract tenant, validate namespace, license lookup |
| quota_test.go | 10 | Consume quota, reset daily, quota exceeded, budget limit |
| sla_test.go | 8 | Record result, calc success rate, detect breach, breach recovery |
| **Total** | **23** | **All unit + integration** |

---

## Deployment

```bash
# 1. Create tenant namespaces
kubectl create namespace agentic-customer-alpha
kubectl create namespace agentic-customer-beta

# 2. Create tenant licenses (as secrets)
kubectl create secret generic customer-alpha-license \
  -n agentic-customer-alpha \
  --from-literal=license.jwt="$LICENSE_ALPHA_JWT"

# 3. Create quotas (as ConfigMap)
kubectl create configmap tenant-quotas \
  -n agentic-system \
  --from-literal=customer-alpha='{"tasksPerDay":100,"costBudgetUSD":1000}'

# 4. Deploy operator (multi-tenant mode)
helm upgrade agentic-operator agentic/agentic-operator \
  --set multitenancy.enabled=true
```

---

## Success Criteria

✅ Extract tenant from workload namespace  
✅ Enforce per-tenant quota (tasks/day)  
✅ Enforce per-tenant cost budget (USD/month)  
✅ Track SLA (success rate %)  
✅ Detect SLA breaches (notify operators)  
✅ Reject workloads when quota exceeded  
✅ Per-tenant Grafana dashboards  
✅ All tests passing  

---

## Next Steps (Phase 8+)

### Phase 8: Auto-Scaling Based on SLA
- Scale LLM provider replicas when success rate drops
- Circuit breaker per tenant
- Fallback to lighter models

### Phase 9: Chargeback & Billing Integration
- Aggregate costs per tenant per month
- Integration with Stripe/Billing
- Usage reports for finance

### Phase 10: Multi-Cluster & Disaster Recovery
- Federated tenants across clusters
- Automatic failover on cluster failure
- Cross-region data replication

---

**Status:** Ready to implement 🚀
