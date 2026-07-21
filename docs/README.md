# Clawdlinux Documentation

Clawdlinux is an in-cluster governance layer for Kubernetes agent workloads.
It does not replace the customer's agent runtime or orchestration system.

## Start Here

- [Quick Start](./01-quickstart.md): install the chart and inspect one workload contract.
- [Installation](./02-installation.md): local chart installation and prerequisites.
- [Configuration](./03-configuration.md): values, runtimes, policy, network, and cost settings.
- [Architecture](./04-architecture.md): current components and runtime adapter contract.
- [Security](./07-security.md): configured controls and enforcement prerequisites.
- [API Reference](./08-api-reference.md): CRD source-of-truth links and implemented semantics.
- [Troubleshooting](./10-troubleshooting.md): common failures and checks.

## Evidence Boundaries

Public demos and documentation use 4 labels:

- `LIVE TODAY`: executed by the current workload path.
- `CONFIGURATION PROOF`: a mutation or policy object exists.
- `PRIOR-RUN PROOF`: a stored fixture is verified now.
- `TARGET PRODUCT`: intended integrated behavior that is not complete.

## Current Repository Scope

Implemented components include:

- AgentWorkload lifecycle and runtime adapters.
- Argo, pod, and kagent runtime registration.
- Admission mutation for labeled gVisor pods.
- Default-deny and optional Cilium policy templates.
- Model routing and cost-reporting interfaces.
- HMAC hash-chain and JSONL verification primitives.
- MCP tools for creating and inspecting AgentWorkloads.

The current controller does not generate a complete signed artifact from every
run. Network enforcement requires the customer's CNI. gVisor requires `runsc`
on the nodes. Rego policy assets are not executed by the direct action path.

## Additional Guides

- [Multi-Tenancy](./05-multi-tenancy.md)
- [Cost Management](./06-cost-management.md)
- [Examples](./09-examples.md): checked-in samples and evidence limits.
- [Monitoring](./11-monitoring.md): exported metrics and optional observability components.
- [Contributing](./12-contributing.md): current Make targets and runtime rules.
- [API Compatibility Policy](./API_COMPATIBILITY_POLICY.md)
- [Security Documentation](./security/README.md)
