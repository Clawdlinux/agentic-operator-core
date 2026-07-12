# Cross-Cluster Agent Identity

> **Status:** Design stage. Implementation gated on validation in [RFC-0001](../rfcs/0001-cross-cluster-agent-identity.md).

This document will hold the operational guide for configuring federated workload identity across Clawdlinux clusters using SPIFFE/SPIRE.

## Current state

Clawdlinux today issues per-cluster identity:

- **Intra-cluster:** Kubernetes ServiceAccount tokens plus an operator-issued JWT for agent-to-agent (A2A) calls.
- **Cross-cluster:** No native mechanism. Operators must share static secrets, run Istio with a shared CA, or accept that agents in Cluster A cannot verifiably authenticate to Cluster B.

## What's coming

[RFC-0001](../rfcs/0001-cross-cluster-agent-identity.md) proposes adopting **SPIFFE/SPIRE** as the workload identity layer for federated identity. Highlights:

- Opt-in per `AgentWorkload` via `spec.identity.spiffe.enabled`
- One trust domain per cluster, federated trust bundles between clusters
- A2A protocol v2 handshake carrying JWT-SVIDs
- Three injection modes for the SPIRE Workload API client (sidecar / init / hostpath)
- Connected (Federation API) and air-gapped (manual bundle exchange) modes
- Existing ServiceAccount-based agents continue working unchanged

## Get involved

Implementation does not start until the [RFC's validation gate](../rfcs/0001-cross-cluster-agent-identity.md#9-validation-gate) clears:

- **6+ external use cases** in the GitHub Discussion, **OR**
- **1 paying customer** request

Comment on the [discussion](https://github.com/Clawdlinux/agentic-operator-core/discussions) or in the [tracking epic](https://github.com/Clawdlinux/agentic-operator-core/issues/146) with your use case.

## See also

- [RFC-0001](../rfcs/0001-cross-cluster-agent-identity.md) — full design proposal
- [Threat model](threat-model.md) — security boundaries this design must preserve
- [A2A architecture](../a2a-architecture.md) — current agent-to-agent protocol
