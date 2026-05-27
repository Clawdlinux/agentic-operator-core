# Roadmap

Public roadmap for NineVigil. Updated quarterly.

## Scope Note

kagent is becoming a strong CNCF base runtime for agents on Kubernetes.

NineVigil will not compete with that layer by default. The open-source core now focuses on regulated controls around agent workloads:

- gVisor runtime isolation
- audit trails
- FinOps attribution
- policy and egress controls
- air-gapped packaging
- ACP integration for MCP context compression

Issues that build generic registries, Envoy routing, sidecar buses, or broad multi-cluster runtime features should move behind validation gates. They stay valid only if a regulated deployment needs them.

## Current (Q1 2026)

- [x] AgentWorkload CRD with full reconciliation lifecycle
- [x] Argo Workflows DAG orchestration
- [x] Cilium FQDN egress policy generation
- [x] LiteLLM proxy integration for multi-provider routing
- [x] MinIO artifact storage per workload
- [x] Multi-tenant namespace isolation with quota enforcement
- [x] Python agent runtime with tool integrations
- [x] A2A (Agent-to-Agent) communication protocol
- [x] Production hardening: staticcheck, secret scanning, CI gates
- [x] Helm chart with subchart dependencies
- [x] Full-cycle integration test suite

## Next (Q2 2026)

- [x] `agentctl` CLI for workload management from terminal
- [x] **MCP server (`agentctl mcp serve`) — agent-callable workload provisioning** ([#140](https://github.com/Clawdlinux/agentic-operator-core/issues/140))
- [ ] Homebrew tap for agentctl
- [x] Agent observability dashboard (Grafana templates)
- [x] Cost dashboard with per-workload token spend visualization
- [ ] Webhook admission controller for CRD validation
- [x] OPA policy library for common agent guardrails
- [x] gVisor RuntimeClass + pod admission injector for labeled agent pods
- [ ] kagent compatibility guide for runtime sandbox labels
- [ ] ACP RemoteMCPServer wrapper example for kagent
- [ ] Air-gapped install smoke test

## Future (Q3-Q4 2026)

- [ ] Multi-cluster federation (validation-gated)
- [ ] GPU-aware scheduling for local model inference (only for regulated local model deployments)
- [ ] Agent evaluation framework (evals-as-code)
- [ ] Managed SaaS offering (hosted control plane)
- [ ] SOC 2 Type II compliance certification
- [ ] Plugin SDK for regulated control extensions
- [ ] Web UI for audit, spend, and sandbox state

## Issues To Re-Scope

These open issues are likely too close to the base runtime layer unless a design partner asks for them:

- #119 Envoy ExtProc capability routing
- #120 Gateway API InferencePool / InferenceModel integration
- #121 Multi-cluster discovery
- #123 NATS/Kafka sidecar injection
- #124 Backpressure signaling
- #97-#112 Agent registry and identity backlog

Keep these as research, not committed roadmap, until they pass a customer validation gate.

## Exploring (RFC stage — not yet committed)

These items have published design documents and are collecting community signal. Implementation begins only after the validation gate in each RFC is met.

### Cross-Cluster Agent Identity Federation (SPIFFE/SPIRE)

- **RFC:** [`docs/rfcs/0001-cross-cluster-agent-identity.md`](docs/rfcs/0001-cross-cluster-agent-identity.md)
- **Discussion:** _(link added when GitHub Discussion is open)_
- **Tentative target:** v0.4.0 (Q3 2026)
- **Validation gate:** 6+ distinct external use cases in the Discussion **OR** 1 paying customer request
- **Motivation:** Enterprise NineVigil deployments span multiple K8s clusters (multi-region, multi-tenant, multi-org). Agents in Cluster A need verifiable identity when calling services or other agents in Cluster B. Existing options (shared secrets, mTLS without workload identity, central OIDC) all fail for air-gapped and regulated environments.
- **Proposed approach:** SPIFFE/SPIRE (CNCF graduated) for workload identity federation. Opt-in per `AgentWorkload`, additive to existing ServiceAccount + JWT identity. A2A protocol gains v2 handshake carrying JWT-SVIDs across trust domains.
- **Triggered by:** [@JacobSobolev on X](https://x.com/JacobSobolev/status/2056631848009085244) (19 May 2026)

## How to Influence the Roadmap

- Open an issue with the `enhancement` label
- Join the discussion in GitHub Discussions
- Submit a PR — we review all contributions

Items are prioritized by community demand and production adoption feedback.
