# Architecture

System design and components.

## Overview

The Agentic Operator is a Kubernetes operator that manages autonomous AI agent workloads at scale.

```
┌──────────────────────────────────────────┐
│     Kubernetes Cluster                   │
├──────────────────────────────────────────┤
│                                          │
│  ┌─────────────────────────────────┐    │
│  │  Agentic System Namespace       │    │
│  │                                 │    │
│  │  ┌──────────────┐               │    │
│  │  │  Operator    │               │    │
│  │  │  (Manager)   │               │    │
│  │  └──────┬───────┘               │    │
│  │         │                       │    │
│  │    ┌────┴─────┐                │    │
│  │    │Controllers                │    │
│  │    ├─Agent Workload Ctrl      │    │
│  │    ├─Tenant Provisioner       │    │
│  │    ├─License Validator        │    │
│  │    └─Cost Tracker             │    │
│  │                                 │    │
│  └─────────────────────────────────┘    │
│                                          │
│  ┌─────────────────────────────────┐    │
│  │  Tenant Namespaces              │    │
│  │  agentic-customer-*             │    │
│  │                                 │    │
│  │  ┌──────────────┐               │    │
│  │  │AgentWorkload │               │    │
│  │  │   Objects    │               │    │
│  │  └──────────────┘               │    │
│  │                                 │    │
│  └─────────────────────────────────┘    │
│                                          │
│  ┌─────────────────────────────────┐    │
│  │  Observability Stack            │    │
│  │  - Prometheus                   │    │
│  │  - Grafana                      │    │
│  │  - Loki                         │    │
│  └─────────────────────────────────┘    │
│                                          │
└──────────────────────────────────────────┘
         │                    │
         ▼                    ▼
    ┌─────────────────────────────────┐
    │   External LLM Providers        │
    │   - OpenAI                      │
    │   - Cloudflare Workers AI       │
    │   - Local Models                │
    └─────────────────────────────────┘
```

## Components

### AgentWorkloadReconciler
Manages individual workload lifecycle:
- Task classification
- Model routing
- Provider execution
- Cost tracking
- Quality evaluation

### TenantReconciler
Provisions and manages tenants:
- Namespace creation
- Secret distribution
- RBAC configuration
- Quota enforcement
- SLA monitoring

### License Validator
Enforces licensing:
- JWT verification
- Tier validation
- Seat limits
- Expiry checks

### Cost Tracker
Tracks token usage:
- Per-provider accounting
- Monthly aggregation
- Quota enforcement
- Billing metrics

## Data Flow

1. **Workload Creation** → AgentWorkload CRD submitted
2. **Validation** → License check, policy evaluation
3. **Classification** → Task categorized (analysis/reasoning/validation)
4. **Routing** → Model selected based on strategy
5. **Execution** → Provider API called
6. **Evaluation** → Quality scored
7. **Completion** → Status updated, metrics recorded

For detailed flows, see respective controller documentation.
