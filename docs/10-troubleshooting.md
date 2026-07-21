# Troubleshooting

## Start With Resource State

```bash
kubectl -n agentic-system get deployments,pods
kubectl -n agentic-system get agentworkloads
kubectl -n agentic-system describe agentworkload <name>
kubectl -n agentic-system get events --sort-by=.lastTimestamp
```

Use the actual Helm release labels when selecting operator logs:

```bash
kubectl -n agentic-system get deployments --show-labels
kubectl -n agentic-system logs deployment/<operator-deployment>
```

## Tenant Stuck In Provisioning

The Tenant controller reads provider Secrets from `agentic-system` using the name
`<provider>-token` and copies them into the tenant namespace.

```bash
kubectl describe tenant <name>
kubectl -n agentic-system get secret <provider>-token
kubectl auth can-i create namespaces \
  --as=system:serviceaccount:agentic-system:<operator-service-account>
```

Also verify permission to create ServiceAccounts, Roles, RoleBindings, Secrets,
and ResourceQuotas.

## Workload Stuck Or Failed

```bash
kubectl -n <namespace> describe agentworkload <name>
kubectl -n <namespace> get agentworkload <name> -o yaml
```

Check:

- referenced provider Secret and key;
- `spec.orchestration.type`;
- `CLAWDLINUX_AGENT_IMAGE` for pod and kagent adapters;
- Argo or kagent installation when selected;
- provider endpoint and model mapping;
- `status.conditions`, `status.argoWorkflow`, and `status.argoPhase`.

## Policy Denial Or Pending Approval

The legacy direct-action path may set `PolicyDenied` or `PendingApproval`.
`opaPolicy` uses an in-process Go evaluator, not Rego execution.

Inspect proposed actions and conditions:

```bash
kubectl -n <namespace> get agentworkload <name> \
  -o jsonpath='{.status.proposedActions}{"\n"}{.status.conditions}{"\n"}'
```

Direct MCP approval continuation is not connected end to end.

## Cost Shows Zero

The default `NoOpCostReporter` records nothing.
For local evaluation, start the operator with the in-memory reporter:

```text
AGENTIC_COST_TRACKING=memory
```

The in-memory reporter resets on restart and is not suitable for billing.

## gVisor Pod Does Not Start

A labeled pod receives `runtimeClassName: gvisor`, but the node must provide a
matching RuntimeClass and `runsc` installation.

```bash
kubectl get runtimeclass gvisor
kubectl -n <namespace> describe pod <pod>
```

## NetworkPolicy Does Not Block Traffic

Confirm:

1. the policy selects the intended pod labels;
2. the policy exists in the intended namespace;
3. the cluster CNI enforces Kubernetes NetworkPolicy;
4. optional Cilium policy is enabled and its selector matches.

Policy-object presence alone does not prove packet enforcement.

## Audit Verification

`audit-verify` currently supports JSONL. The ClickHouse source adapter is a stub.
The controller does not automatically emit a signed entry for every run.

Use the checked-in fixture only as prior-run proof. See
[SHOWCASE-DEMO-WALKTHROUGH.md](./SHOWCASE-DEMO-WALKTHROUGH.md).
