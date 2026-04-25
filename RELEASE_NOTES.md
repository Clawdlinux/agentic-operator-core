# Agentic Operator v0.2.0 Release Notes

**Release Date:** April 25, 2026

---

## What's New in v0.2.0

This release adds production-ready multi-agent demos, a pluggable workflow engine, CLI onboarding, and per-workload cost attribution — everything needed to deploy and manage AI agent fleets on Kubernetes in air-gapped environments.

---

## ✨ Highlights

### Multi-Agent Demos (Production-Ready)
- **Research Swarm** — A2A agent discovery, persona tool_profile blocking, OPA budget enforcement, autoApproveThreshold gating. Docker Compose with ollama local overlay for fully offline operation.
- **SRE Incident Response** — `collaborationMode: delegation`, 3-tier cost-aware model routing (cheap triage → mid analysis → expensive remediation), adversarial tone on remediator, hierarchical memory. 4 alert scenarios included.

### Pluggable Workflow Registry
- `@register_workflow` decorator for custom LangGraph workflows
- Auto-discovery from built-in + custom directories
- `WORKFLOW_NAME` env var selects workflow at runtime
- Ships with 3 workflows: research_swarm, code_review (fan-out security/performance/style), doc_processor (parallel entity extraction + summarization)

### Agent FinOps
- **MemoryCostReporter** with Prometheus metrics (`agentic_workload_cost_usd`, `agentic_workload_tokens_total`)
- Pre-loaded pricing for OpenAI, Anthropic, Azure OpenAI, and Ollama (local = $0)
- Budget enforcement with configurable thresholds
- Per-workload cost annotations scraped by Prometheus

### agentctl CLI Expansion
- `agentctl init` — Interactive cluster onboarding wizard
- `agentctl approve` — Resume PendingApproval workloads via annotation patch + Argo resume
- `agentctl workflows` — List all registered workflows
- `agentctl status` — Cluster health overview
- Total: **10 commands** (init, apply, get, describe, logs, cost, approve, workflows, status, version)

### CRD Enhancements
- `spec.workflowName` field on AgentWorkload for workflow selection
- deepcopy regenerated for all CRD changes

---

## 🚀 Quick Start

### Multi-Agent Swarm (fully offline with Ollama)

```bash
cd examples/multi-agent-swarm
cp .env.example .env        # add your API key (or use Ollama)
make up                     # starts 7 containers
make demo                   # runs full 6-stage pipeline
```

### SRE Incident Response

```bash
cd examples/sre-incident-response
cp .env.example .env
docker compose up -d
curl -X POST http://localhost:9010/incidents \
  -H "Content-Type: application/json" \
  -d '{"alert_type":"PodCrashLoopBackOff","namespace":"default","resource":"app-server-7b9f4c6d8-x2k9p"}'
```

### CLI Onboarding

```bash
agentctl init               # interactive cluster setup
agentctl status             # check cluster health
agentctl workflows          # list available workflows
```

---

## 📦 Installation

### Helm

```bash
helm repo add agentic https://clawdlinux.github.io/agentic-operator-core
helm install agentic-operator agentic/agentic-operator
```

### agentctl

```bash
# macOS
curl -sL https://github.com/Clawdlinux/agentic-operator-core/releases/download/v0.2.0/agentctl-darwin-arm64 -o agentctl
chmod +x agentctl && sudo mv agentctl /usr/local/bin/
```

---

## 🔗 Links

- **GitHub:** https://github.com/Clawdlinux/agentic-operator-core
- **Landing:** https://clawdlinux.org
- **Docs:** https://github.com/Clawdlinux/agentic-operator-core/tree/main/docs

---

## Full Changelog

See [CHANGELOG.md](CHANGELOG.md) for the complete list of changes since v0.1.1.

# 4. Run the demo pipeline
make run-demo
```

Expected output:
- Research agent gathers information
- Writer agent drafts the article
- Editor agent polishes and formats
- Total cost: ~$0.02 USD

See `examples/research-swarm/QUICKSTART.md` for full details.

---

## 📦 What's Included

### Core Packages
- `pkg/evaluation/` — Evaluation framework, metrics, and scorers
- `pkg/mcp/` — Native MCP protocol client and mock server
- `pkg/resilience/` — Circuit breakers, retry policies, deadlines
- `pkg/metrics/` — Agent-level cost, latency, error rate tracking
- `pkg/multitenancy/` — Tenant isolation and RBAC
- `pkg/autoscaling/` — Dynamic scaling based on demand
- `enterprise/billing/` — Cost tracking and enforcement
- `enterprise/licensing/` — License validation and policy enforcement

### CRDs
- `AgentWorkload` — Run multi-agent pipelines in Kubernetes
- `AgentPersona` — Define agent identity and capabilities
- `AgentCard` — A2A agent discovery and advertisement
- `Tenant` — Multi-tenant isolation and quotas

### CLI
- `agentctl` — Agent lifecycle management (6 subcommands, multiple output formats)
- Binary releases for linux/amd64, linux/arm64, darwin/amd64, darwin/arm64

### Examples
- `examples/research-swarm/` — Complete research-write-edit pipeline with Docker Compose
- K8s manifests for production deployment
- Helm chart with sensible defaults

---

## 🔧 System Requirements

**Minimum (Local Development):**
- Docker Engine 20.10+
- Docker Compose 2.0+
- 8 GB RAM, 4 CPU cores
- OpenAI API key (or compatible LLM)

**Production (Kubernetes):**
- Kubernetes 1.24+
- cert-manager (for webhook TLS)
- PostgreSQL 13+ (for audit logs and spans)
- MinIO or S3-compatible storage (for artifacts)

---

## 🔒 Security

This release includes:
- HTTPS-only enforcement for MCP endpoints
- Webhook TLS via cert-manager
- Secret preservation across Helm upgrades
- RBAC with least-privilege default policies
- Removed dangerous wildcard cluster roles
- Repository secret scanning in CI/CD

See [SECURITY.md](SECURITY.md) for vulnerability disclosure policy.

---

## 📈 Performance

Typical research-write-edit pipeline:
- **Total latency:** 3-5 minutes depending on LLM
- **Throughput:** 3-5 concurrent pipelines per agent
- **Cost per pipeline:** ~$0.02 USD (varies by model)
- **Memory per agent:** 256-512 MB
- **CPU per agent:** 100-500m

---

## 🙏 Acknowledgments

Special thanks to the design partners who shaped the early vision:
- Agent framework maintainers for MCP protocol leadership
- Kubernetes community for adoption feedback
- Production operators who tested resilience patterns

---

## 📝 What's Next

Planned for v0.2.0:
- Distributed tracing integration (Jaeger/Tempo)
- Grafana dashboard templates
- Agent callback webhooks for event-driven workflows
- Automatic model routing based on cost/capability
- Policy-based governance templates

---

## 🤝 Get Involved

- **GitHub:** [agentic-operator-core](https://github.com/clawdlinux/agentic-operator-core)
- **Issues:** Report bugs or request features
- **Discussions:** Questions and feedback
- **Contributing:** See CONTRIBUTING.md

---

## 📄 License

Apache License 2.0 — See LICENSE for details.

---

## 🙌 Download

### Binaries
- [agentctl-linux-amd64](https://github.com/clawdlinux/agentic-operator-core/releases/download/v0.1.0/agentctl-linux-amd64)
- [agentctl-linux-arm64](https://github.com/clawdlinux/agentic-operator-core/releases/download/v0.1.0/agentctl-linux-arm64)
- [agentctl-darwin-amd64](https://github.com/clawdlinux/agentic-operator-core/releases/download/v0.1.0/agentctl-darwin-amd64)
- [agentctl-darwin-arm64](https://github.com/clawdlinux/agentic-operator-core/releases/download/v0.1.0/agentctl-darwin-arm64)

### Container Images
- `clawdlinux/agentic-operator:v0.1.0` — Controller + webhooks
- `clawdlinux/agentic-agent-base:v0.1.0` — Base agent image

### Helm Chart
```bash
helm repo add agentic https://charts.clawdlinux.org
helm repo update
helm install agentic agentic/agentic-operator --version 0.1.0
```
