# Quick Start

Install Clawdlinux from the local umbrella chart and inspect one workload.

## Prerequisites

- Kubernetes 1.27 or later.
- Helm 3.
- `kubectl` configured for the target cluster.
- A container registry or locally loaded operator image.
- Optional model credentials for workloads that call an external provider.

## 1. Clone And Validate

```bash
git clone https://github.com/Clawdlinux/agentic-operator-core.git
cd agentic-operator-core

helm dependency build ./charts
helm lint ./charts
```

## 2. Install

```bash
helm upgrade --install clawdlinux ./charts \
  --namespace agentic-system \
  --create-namespace \
  --set license.key=dev-only-not-a-valid-license
```

The chart currently requires a nonempty packaging value. The open-source
reconciler uses a no-op validator, so this development value is not a license or
security control. Never use it for a production deployment.

## 3. Verify Components

```bash
kubectl -n agentic-system get deployments,pods
kubectl get crd agentworkloads.agentic.clawdlinux.org
kubectl -n agentic-system get deployments
```

Deployment names depend on the Helm release name. Select the operator deployment
from the final command before reading its logs.

## 4. Review A Workload Contract

Start with a sample and inspect it before applying:

```bash
kubectl apply --dry-run=server \
  -f config/samples/agentworkload_demo.yaml
```

Check these fields:

- `spec.objective`
- `spec.orchestration.type`
- `spec.providers` and `spec.modelMapping`
- `spec.persona.toolProfile`
- resource and timeout limits
- policy mode and approval settings

Apply only after configuring the referenced provider endpoint and Secret:

```bash
kubectl apply -f config/samples/agentworkload_demo.yaml
kubectl -n agentic-system get agentworkloads -w
```

## What This Proves

The quickstart proves chart installation, CRD availability, admission, and the
selected runtime path. It does not prove packet enforcement, full air-gap
operation, same-run signed evidence, or compliance with a named framework.

For the evidence demo, follow
[SHOWCASE-DEMO-WALKTHROUGH.md](./SHOWCASE-DEMO-WALKTHROUGH.md).

## Next Steps

- [Configuration](./03-configuration.md)
- [Architecture](./04-architecture.md)
- [Security](./07-security.md)
- [Cost Management](./06-cost-management.md)
