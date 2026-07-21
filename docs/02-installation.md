# Installation

This repository currently supports installation from its local Helm chart.
Do not use `helm.agentic.io`; no public chart repository exists at that address.

## Requirements

- Kubernetes 1.27 or later.
- Helm 3.
- `kubectl` access with permission to create CRDs and cluster-scoped RBAC.
- Storage classes for any enabled stateful subcharts.
- Network access or mirrored images for enabled components.

Optional controls require additional cluster support:

- gVisor mutation requires `runsc` and a working `RuntimeClass`.
- NetworkPolicy enforcement requires an enforcing CNI.
- Cilium FQDN policy requires Cilium.
- External model calls require approved credentials and egress.

## Install From Source

```bash
git clone https://github.com/Clawdlinux/agentic-operator-core.git
cd agentic-operator-core

helm dependency build ./charts
helm upgrade --install clawdlinux ./charts \
  --namespace agentic-system \
  --create-namespace \
  --set license.key=dev-only-not-a-valid-license
```

The chart currently requires a nonempty `license.key`. The open-source
reconciler defaults to a no-op validator. Use the development value only for
local evaluation.

Use a reviewed values file for production:

```bash
helm upgrade --install clawdlinux ./charts \
  --namespace agentic-system \
  --create-namespace \
  --values ./my-values.yaml \
  --set license.key="$CLAWDLINUX_LICENSE_JWT"
```

Production license tokens can be generated with `cmd/generate-license` when the
required signing material and commercial terms are available. Do not commit a
token to a values file.

## Verify

```bash
helm status clawdlinux -n agentic-system
kubectl -n agentic-system get deployments,pods,services
kubectl get crd | grep agentic.clawdlinux.org
```

## Air-Gapped Environments

The chart supports image repository overrides and offline JWT validation.
A production air-gap deployment must mirror every enabled image and dependency.
The repository does not yet run a complete air-gap installation smoke test in CI.

## Uninstall

```bash
helm uninstall clawdlinux -n agentic-system
```

Helm does not automatically delete CRDs or persistent volumes. Review retained
resources before deleting namespaces or storage.

See [Configuration](./03-configuration.md) before changing runtime, network,
storage, model, or observability settings.
