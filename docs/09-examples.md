# Examples

Use checked-in examples instead of copying stale provider or pricing values from documentation.

## Governance And Demo Samples

- [`agentworkload_demo.yaml`](../config/samples/agentworkload_demo.yaml)
- [`agentworkload_demo_allow.yaml`](../config/samples/agentworkload_demo_allow.yaml)
- [`agentworkload_demo_deny.yaml`](../config/samples/agentworkload_demo_deny.yaml)
- [`agentworkload_demo_runtime.yaml`](../config/samples/agentworkload_demo_runtime.yaml)
- [`agentworkload-cost-aware-routing.yaml`](../config/samples/agentworkload-cost-aware-routing.yaml)

## Collaboration Samples

- [`agentworkload-persona-swarm.yaml`](../config/samples/agentworkload-persona-swarm.yaml)
- [`agentworkload_demo_swarm.yaml`](../config/samples/agentworkload_demo_swarm.yaml)
- [`a2a_team_example.yaml`](../config/samples/a2a_team_example.yaml)

## Workflow Examples

- [`research-swarm.yaml`](../config/examples/research-swarm.yaml)
- [`code-review.yaml`](../config/examples/code-review.yaml)
- [`doc-processor.yaml`](../config/examples/doc-processor.yaml)

## Safe Evaluation Pattern

Review and dry-run a sample before applying it:

```bash
kubectl apply --dry-run=server \
  -f config/samples/agentworkload_demo.yaml
```

Check provider endpoints, Secret references, runtime type, tool profile, policy
mode, resources, and timeouts.

Apply only after replacing placeholders:

```bash
kubectl apply -f config/samples/agentworkload_demo.yaml
kubectl -n agentic-system get agentworkloads -w
```

## Important Limits

- Sample confidence values are inputs to the Go policy evaluator. They are not an independent security score.
- Pricing and cost outputs are estimates. Validate them against current provider pricing.
- Tenant monthly token fields are not automatically aggregated by the Tenant reconciler.
- NetworkPolicy and gVisor samples prove configuration. Enforcement requires CNI and node-runtime support.
- The current demo verifies a prior-run audit fixture. It does not produce same-run signed evidence.
