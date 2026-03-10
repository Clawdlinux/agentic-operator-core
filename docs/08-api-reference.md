# API Reference

Complete CRD specification.

For versioning guarantees, deprecation windows, and breaking-change governance, see [API Compatibility Policy](./API_COMPATIBILITY_POLICY.md).

## Tenant CRD

### Spec

```yaml
apiVersion: agentic.clawdlinux.org/v1alpha1
kind: Tenant
metadata:
  name: <tenant-name>
spec:
  displayName: <string>              # Human-readable name
  namespace: <string>                # K8s namespace
  providers: [<string>]              # LLM providers
  quotas:
    maxWorkloads: <int>              # Concurrent workloads
    maxConcurrent: <int>             # Parallel executions
    maxMonthlyTokens: <int64>        # Token budget
    cpuLimit: <string>               # CPU cores
    memoryLimit: <string>            # RAM
  slaTarget: <float>                 # SLA percentage
  networkPolicy: <bool>              # Enable isolation
```

### Status

```yaml
status:
  phase: <Pending|Provisioning|Active|Failed|Terminating>
  conditions: [<Condition>]          # Ready, NamespaceCreated, etc
  namespaceCreated: <bool>
  secretsProvisioned: <bool>
  rbacConfigured: <bool>
  quotasEnforced: <bool>
  networkPolicyActive: <bool>
  workloadCount: <int>
  tokensUsedThisMonth: <int64>
  lastReconciliation: <Time>
```

## AgentWorkload CRD

See `Examples` for complete specifications.

### Fields

- `objective` - Task description
- `modelStrategy` - fixed|cost-aware
- `taskClassifier` - default
- `autoApproveThreshold` - Quality threshold
- `providers` - LLM provider configurations
- `modelMapping` - Task category → model mapping
- `opaPolicy` - strict|permissive

### Status

- `phase` - Pending|Processing|Completed|Failed
- `conditions` - Detailed status
- `tokensUsed` - Input/output token count

See full API at `/api/v1alpha1`.
