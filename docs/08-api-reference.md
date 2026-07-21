# API Reference

The generated CRDs are the source of truth:

- [`AgentWorkload`](../config/crd/bases/agentic.clawdlinux.org_agentworkloads.yaml)
- [`AgentCard`](../config/crd/bases/agentic.clawdlinux.org_agentcards.yaml)
- [`Tenant`](../config/crd/bases/agentic.clawdlinux.org_tenants.yaml)

See [API Compatibility Policy](./API_COMPATIBILITY_POLICY.md) for versioning and deprecation rules.

## AgentWorkload

Important spec groups:

- Objective and legacy workflow: `objective`, `agents`, `workflowName`, `jobId`.
- Runtime: `orchestration.type`, `workflowTemplateRef`.
- Limits: `resources`, `timeouts`.
- Models: `modelStrategy`, `taskClassifier`, `providers`, `modelMapping`.
- Actions: `autoApproveThreshold`, `opaPolicy`.
- Collaboration: `collaborationMode`, `agentRefs`, `persona`.
- External integration: `mcpServerEndpoint`.

Registered runtime types are `argo`, `pod`, and `kagent`.

`opaPolicy` selects strict or permissive behavior in the legacy in-process Go
evaluator. It does not cause the direct action path to execute Rego.

Important status fields:

- `phase`
- `conditions`
- `proposedActions`
- `executedActions`
- `argoWorkflow` and `argoPhase`
- `workflowArtifacts`
- `modelRoutingOperationID`
- `agentStatuses`

The historical `argoWorkflow` field also stores pod and kagent execution references.
Its `runtime` field identifies the selected adapter.

## Tenant

The Tenant spec includes namespace, provider names, resource quotas, an SLA target,
and a network-policy flag.

The current tenant reconciler creates:

- the tenant namespace;
- copies of named provider Secrets from `agentic-system`;
- a service account, Role, and RoleBinding;
- CPU, memory, and pod-count ResourceQuota entries.

`maxConcurrent`, `maxMonthlyTokens`, `slaTarget`, and `networkPolicy` are API fields,
but the current Tenant reconciler does not provide complete enforcement for them.

Some Tenant status fields are reserved for later integrations and may remain at
their zero value.

## AgentCard

AgentCard describes reusable capability metadata and endpoints. It does not by
itself bind an external employee identity to an AgentWorkload request.

## MCP Surface

`agentctl mcp serve` exposes tools for creating, listing, inspecting, and deleting
AgentWorkloads. See [agentctl MCP](./agentctl/mcp.md).

Bearer-token authentication protects the trusted adapter. Individual actor
identity propagation and full RBAC remain integration work.
