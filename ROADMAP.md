# Roadmap

Public roadmap for the Agentic Kubernetes Operator. Updated quarterly.

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

- [ ] `agentctl` CLI for workload management from terminal
- [ ] Homebrew tap for agentctl
- [ ] Agent observability dashboard (Grafana templates)
- [ ] Cost dashboard with per-workload token spend visualization
- [ ] Webhook admission controller for CRD validation
- [ ] OPA policy library for common agent guardrails
- [ ] Agent marketplace: community-contributed agent templates
- [ ] Horizontal pod autoscaling based on queue depth

## Future (Q3–Q4 2026)

- [ ] Multi-cluster federation (workloads span clusters)
- [ ] GPU-aware scheduling for local model inference
- [ ] Agent evaluation framework (evals-as-code)
- [ ] Managed SaaS offering (hosted control plane)
- [ ] SOC 2 Type II compliance certification
- [ ] Plugin SDK for custom tool integrations
- [ ] Visual workflow builder (web UI)

## How to Influence the Roadmap

- Open an issue with the `enhancement` label
- Join the discussion in GitHub Discussions
- Submit a PR — we review all contributions

Items are prioritized by community demand and production adoption feedback.
