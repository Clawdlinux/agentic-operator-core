# Multi-Tenancy Guide

Complete guide to multi-tenant provisioning and management.

## Overview

The Tenant CRD enables automatic provisioning of complete tenant environments:
- Isolated Kubernetes namespaces
- Per-tenant secrets for provider access
- RBAC with service accounts and roles
- Resource quotas and limits
- SLA monitoring

## Tenant Lifecycle

### 1. Create Tenant

```yaml
apiVersion: agentic.clawdlinux.org/v1alpha1
kind: Tenant
metadata:
  name: acme-corp
spec:
  displayName: "ACME Corporation"
  namespace: agentic-customer-acme
  providers:
    - cloudflare-workers-ai
    - openai
  quotas:
    maxWorkloads: 100
    maxConcurrent: 10
    maxMonthlyTokens: 10000000
    cpuLimit: "10"
    memoryLimit: "20Gi"
  slaTarget: 99.5
  networkPolicy: true
```

Apply:
```bash
kubectl apply -f tenant-acme.yaml
```

### 2. Monitor Provisioning

```bash
kubectl get tenants
kubectl describe tenant acme-corp
kubectl get tenants acme-corp --watch
```

Watch for:
- Phase: Pending → Provisioning → Active
- NamespaceCreated: true
- SecretsProvisioned: true
- RBACConfigured: true
- QuotasEnforced: true

### 3. Deploy Workloads

Once tenant status is "Active", deploy workloads:

```bash
kubectl apply -f workload.yaml -n agentic-customer-acme
```

## Tenant Isolation

### Namespace Isolation
Each tenant gets exclusive namespace:
```bash
agentic-customer-acme (tenant: acme-corp)
agentic-customer-bigco (tenant: bigco)
agentic-customer-startup (tenant: startup)
```

### RBAC Isolation
Service accounts limited to their namespace:
```bash
acme-corp-agent (can manage workloads in agentic-customer-acme only)
bigco-agent (can manage workloads in agentic-customer-bigco only)
```

### Secret Isolation
Secrets copied to tenant namespace:
```bash
agentic-system/cloudflare-workers-ai-token
↓
agentic-customer-acme/cloudflare-workers-ai-token (copy)
```

### Network Isolation (Optional)
NetworkPolicy restricts inter-namespace traffic:
```yaml
kind: NetworkPolicy
metadata:
  name: acme-isolation
  namespace: agentic-customer-acme
spec:
  podSelector: {}
  policyTypes:
    - Ingress
    - Egress
  ingress:
    - from:
        - namespaceSelector:
            matchLabels:
              name: agentic-customer-acme
  egress:
    - to:
        - namespaceSelector: {}
```

## Quota Management

### Per-Tenant Quotas

```yaml
quotas:
  maxWorkloads: 100         # Max concurrent AgentWorkloads
  maxConcurrent: 10         # Max concurrent executions
  maxMonthlyTokens: 10M     # Max tokens per month
  cpuLimit: "10"            # CPU cores
  memoryLimit: "20Gi"       # RAM
```

### Enforcement

Operator enforces quotas:
- **Pod limit**: Can't create more pods than maxWorkloads
- **CPU/Memory**: Limited by ResourceQuota
- **Monthly tokens**: Tracked and enforced monthly

View quota usage:
```bash
kubectl get resourcequota -n agentic-customer-acme
kubectl describe resourcequota acme-corp-quota -n agentic-customer-acme
```

## Cost Attribution

Track per-tenant costs:
```bash
kubectl get tenant acme-corp -o json | jq '.status.tokensUsedThisMonth'
```

Costs are automatically calculated per provider and aggregated.

## Delete Tenant

```bash
kubectl delete tenant acme-corp
```

This removes:
- ✅ Namespace and all workloads
- ✅ RBAC roles and bindings
- ✅ Resource quotas
- ✅ Secrets

## Best Practices

1. **Use descriptive names** - `acme-corp`, not `tenant1`
2. **Set realistic quotas** - Don't set limits too low
3. **Monitor SLA** - Set appropriate SLATarget
4. **Enable NetworkPolicy** - For production multi-tenancy
5. **Rotate secrets** - Update provider tokens quarterly

For examples, see `Examples`.
