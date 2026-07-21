# Configuration

Use [`charts/values.yaml`](../charts/values.yaml) and
[`charts/values.schema.json`](../charts/values.schema.json) as the source of
truth for Helm settings.

## Runtime Selection

`AgentWorkload.spec.orchestration.type` selects a registered runtime:

- `argo`: default multi-step DAG execution.
- `pod`: bring-your-own single pod.
- `kagent`: unstructured `kagent.dev/v1alpha2` Agent adapter.

Adding a runtime requires a `runtime.RuntimeAdapter` implementation and registry
registration. Do not add runtime-specific branches to the reconciler.

## Runtime Sandbox

The admission webhook mutates labeled pods:

```yaml
metadata:
  labels:
    agentic.clawdlinux.org/runtime-sandbox: gvisor
```

The pod receives `runtimeClassName: gvisor` when it does not already specify a
runtime class. The nodes must have a working `runsc` installation.

## Network Policy

The umbrella chart renders a default-deny egress policy when
`networkPolicy.enabled=true`. It selects pods in the release namespace by label.

Optional Cilium FQDN policy requires:

```yaml
networkPolicy:
  cilium:
    enabled: true
```

Policy objects are configuration. Packet enforcement depends on the cluster CNI.

## Action Policy

`AgentWorkload.spec.opaPolicy` selects strict or permissive behavior in the
legacy in-process Go evaluator. The direct action path does not execute Rego.

Rego samples under `pkg/opa` and `config/policies` are integration assets.

## Model Routing

Provider endpoints and Secret references belong in the AgentWorkload spec.
LiteLLM may be enabled as an in-cluster multi-provider proxy.

Never place provider keys in a ConfigMap or committed values file.

## Cost Reporting

The open-source operator defaults to `NoOpCostReporter`.
Enable the in-memory reporter only for local evaluation:

```text
AGENTIC_COST_TRACKING=memory
```

The in-memory reporter is volatile and unsuitable for billing or chargeback.

## Licensing

Offline JWT validation is available when a production validator is configured.
The open-source reconciler defaults to a no-op validator.

## Observability

The optional observability chart deploys OTel Collector, Tempo, Prometheus,
Grafana, ClickHouse, and Qdrant components. Audit table creation does not mean
the controller writes same-run audit rows.

See [Security](./07-security.md), [Cost Management](./06-cost-management.md),
and [Architecture](./04-architecture.md) before production use.
