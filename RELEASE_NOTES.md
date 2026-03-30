# Agentic Operator v0.1.1 Release Notes

**Release Date:** March 31, 2026

---

## Welcome to the Agentic Operator

The Agentic Operator is the **first Kubernetes-native agent orchestration framework** designed for production multi-agent AI systems. It brings first-class support for agent lifecycle management, observability, evaluation, and enterprise governance directly into Kubernetes.

---

## ✨ What's New in v0.1.1

### Kubernetes-Native Agent Management
- **AgentPersona CRD** — Define agent identity, memory scope, system prompts, and tool profiles as Kubernetes resources
- **agentctl CLI** — Complete lifecycle management: get, describe, logs, cost, apply, version with table/JSON/YAML output
- **Multitenancy** — Tenant isolation, RBAC, quotas for shared Kubernetes clusters
- **Resilience** — Circuit breakers, retry policies, deadline management for fault-tolerant agent pipelines

### Built-In Evaluation Framework
- **Agent Quality Metrics** — Accuracy, consistency, latency, cost tracking per agent
- **Scorer Interface** — Pluggable evaluation backends for custom metrics
- **Cost Accounting** — Per-agent spend tracking and FinOps enforcement
- **Production-Ready** — Already addresses the single biggest enterprise deployment bottleneck

### Agent-to-Agent Communication
- **AgentCard CRD** — A2A-compatible agent discovery (role, capabilities, endpoint, auth, health)
- **MCP Protocol Client** — Native support for Model Context Protocol (97M monthly SDK downloads)
- **Agent Discovery** — Kubernetes-native answer to agent DNS problem
- **Multi-Tenant Visibility** — Agents across clusters can discover each other in real-time

### Developer Experience
- **Research Swarm Quickstart** — 4-command Docker Compose demo (research → write → edit pipeline)
- **Comprehensive CLI** — Query cost, logs, metrics without kubectl context switching
- **Helm Integration** — Production-ready charts with auto-generated secrets and job templates

### Enterprise Infrastructure
- **FinOps Packages** — Billing, licensing, cost enforcement for multi-tenant scenarios
- **Observability** — Prometheus metrics, structured logging, distributed tracing hooks
- **Autoscaling** — Dynamic agent pool scaling based on queue depth and workload metrics

---

## 🚀 Quick Start

Get started in 4 commands:

```bash
cd examples/research-swarm

# 1. Copy environment and add your OpenAI API key
cp .env.example .env
# Edit .env with your OPENAI_API_KEY

# 2. Build Docker images
make build

# 3. Start the full stack
make up

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
