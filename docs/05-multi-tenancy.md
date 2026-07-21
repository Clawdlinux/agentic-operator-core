# Multi-Tenancy Guide

Complete guide to multi-tenant provisioning and management.

## Overview

The Tenant controller provisions selected namespace resources:
- Isolated Kubernetes namespaces
- Per-tenant secrets for provider access
- RBAC with service accounts and roles
- Resource quotas and limits
- Tenant status fields for later operational integrations

Review the generated RBAC before production use. The tenant service account can
list Secrets in its namespace.

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

### Network Isolation

The Tenant CRD exposes `spec.networkPolicy`, but the current tenant reconciler
does not create the following policy automatically. Treat this as an example:
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

The tenant reconciler creates a Kubernetes `ResourceQuota`:
- `maxWorkloads` maps to a pod-count quota.
- `cpuLimit` maps to total CPU quota.
- `memoryLimit` maps to total memory quota.

`maxConcurrent` and `maxMonthlyTokens` are not enforced by this ResourceQuota.

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

The current tenant reconciler does not populate that field. Use a durable
`CostReporter` and aggregation integration before relying on tenant chargeback.

## Delete Tenant

```bash
kubectl delete tenant acme-corp
```

The current reconciler does not implement finalizer-based cleanup. Deleting a
Tenant does not guarantee deletion of its namespace, RBAC, quotas, or Secrets.
Inventory and remove retained resources explicitly.

## Best Practices

1. **Use descriptive names** - `acme-corp`, not `tenant1`
2. **Set realistic quotas** - Don't set limits too low
3. **Define SLA monitoring externally** - `slaTarget` is not a complete SLO system
4. **Apply NetworkPolicy explicitly** - Confirm the CNI enforces it
5. **Rotate secrets** - Update provider tokens quarterly

For examples, see `Examples`.
