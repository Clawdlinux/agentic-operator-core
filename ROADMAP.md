# Roadmap

Public roadmap for Clawdlinux. Updated quarterly. For the architecture and use case, read [docs/DESIGN.md](docs/DESIGN.md) first.

## The problem we are solving

Enterprises in regulated industries cannot ship AI agents to production. Not because the agents do not work, but because nobody can answer the governance questions: who is the agent acting as, what can it reach, what did it cost, and can an auditor verify what it did after the fact. Gartner expects over 40% of agentic AI projects to be canceled by end of 2027, with inadequate risk controls named as a cause. The runtimes are getting good. The controls around them are the gap.

## The vision

The target product makes each governed Kubernetes agent run leave signed, offline-verifiable evidence and execute inside a declared egress boundary. Clawdlinux is building that runtime-agnostic governance plane for customer-owned clusters.

The current repository ships separate audit, admission, network-policy, runtime,
and cost components. Same-run capture and enforcement parity are not complete.

## How we got here

- **Feb 2026.** Project started as an enterprise agent-swarm orchestration platform (operator, AgentWorkload CRD, Argo DAGs, multi-tenancy, licensing, cost tracking). Initial wedge idea: visual market analysis for hedge funds needing zero data leakage.
- **Mar to Apr 2026.** Core platform hardened: Cilium FQDN egress generation, LiteLLM routing, MinIO artifacts, Python LangGraph runtime with A2A, full-cycle integration tests, Helm umbrella chart.
- **May 2026.** ACP (Agent Contract Protocol) spun out as its own repo and spec. Benchmarks landed: 64.7% to 97.4% token reduction vs raw MCP, one round trip. Briefly explored a consumer AgentOS direction; reversed within the month. Enterprise K8s is the business.
- **Jun 2026.** Positioning locked: we sell the governance and evidence plane, not a runtime. Runtime adapters, gVisor mutation, and offline JSONL verification primitives shipped. Governance parity and same-run capture remained incomplete.
- **Jul 2026.** Demo gate (`scripts/demo-booth.sh`) reproducible on kind. Focus: validation conversations and the Jul 22 Agentic Summit booth. Fintech is the anchor vertical.

## Scope note

Strong CNCF base runtimes for agents on Kubernetes exist and are improving. Clawdlinux does not compete with that layer. The open-source core focuses on regulated controls around agent workloads: gVisor isolation, audit trails, FinOps attribution, policy and egress controls, air-gapped packaging, and ACP integration. Issues that build generic registries, Envoy routing, sidecar buses, or broad multi-cluster runtime features stay behind validation gates. They are valid only if a regulated deployment needs them.

## Shipped (Q1 2026)

- [x] AgentWorkload CRD with full reconciliation lifecycle
- [x] Argo Workflows DAG orchestration
- [x] Cilium FQDN egress policy generation
- [x] LiteLLM proxy integration for multi-provider routing
- [x] MinIO artifact storage per workload
- [x] Multi-tenant namespace isolation with quota enforcement
- [x] Python agent runtime with tool integrations
- [x] A2A (Agent-to-Agent) communication protocol
- [x] Quality gates: staticcheck, secret scanning, and CI checks
- [x] Helm chart with subchart dependencies
- [x] Full-cycle integration test suite

## Shipped (Q2 2026)

- [x] `agentctl` CLI for workload management
- [x] MCP server (`agentctl mcp serve`) for agent-callable provisioning ([#140](https://github.com/Clawdlinux/agentic-operator-core/issues/140))
- [x] HMAC hash-chain and offline JSONL `audit-verify`
- [x] gVisor RuntimeClass + pod admission injector for labeled agent pods
- [x] Runtime adapter interface (AgentWorkload, BYO pods, external runtimes)
- [x] Rego policy samples and an in-process Go action evaluator
- [x] Grafana observability and per-workload cost dashboards
- [x] Reproducible booth demo gate on kind

## Now (Q3 2026)

Priority order. Validation before features.

- [ ] 10 qualified platform/security conversations by Sep 15, including 3 active production-review blockers and 1 scoped design-partner engagement. This is the one-time reset of the missed Jul 15 criterion.
- [ ] Jul 22 Agentic Summit booth demo, fintech use case
- [ ] Air-gapped install smoke test in CI
- [ ] Webhook admission controller for CRD validation
- [ ] Per-runtime sandbox label guide
- [ ] Same-run audit capture with durable storage and production key custody
- [ ] Runtime governance-label parity and enforcing-CNI packet tests
- [ ] ACP RemoteMCPServer wrapper example
- [ ] Homebrew tap for agentctl

## Later (Q4 2026+, all validation-gated)

- [ ] Cross-cluster agent identity federation (SPIFFE/SPIRE). RFC: [docs/rfcs/0001-cross-cluster-agent-identity.md](docs/rfcs/0001-cross-cluster-agent-identity.md). Gate: 6+ distinct external use cases or 1 paying customer request.
- [ ] Multi-cluster federation
- [ ] GPU-aware scheduling for local model inference (regulated local-model deployments only)
- [ ] Agent evaluation framework (evals-as-code)
- [ ] SOC 2 Type II
- [ ] Web UI for audit, spend, and sandbox state
- [ ] Managed SaaS offering (hosted control plane)
- [ ] Plugin SDK for regulated control extensions

## Issues to re-scope

Too close to the base runtime layer unless a design partner asks: #119 Envoy ExtProc routing, #120 Gateway API InferencePool, #121 multi-cluster discovery, #123 NATS/Kafka sidecars, #124 backpressure, #97-#112 registry/identity backlog. Research, not committed roadmap, until they pass a customer validation gate.

## How to influence the roadmap

Open an issue with the `enhancement` label, join GitHub Discussions, or send a PR. Items are prioritized by community demand and production deployment feedback. Anything a regulated design partner needs jumps the queue.
