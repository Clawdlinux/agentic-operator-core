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


---

## 🔒 Security

- HTTPS-only enforcement for MCP endpoints
- Webhook TLS via cert-manager, `failurePolicy: Fail`
- Distroless containers, non-root user
- Credential sanitizer in logs (OpenAI, GitHub, AWS, JWT, bearer tokens)
- Prompt injection sanitization on scraped content
- OPA-gated action execution with default-deny policies
- See [SECURITY.md](SECURITY.md) for vulnerability disclosure (48h SLA)

---

## 📦 Downloads

Binaries are attached to this release. Pick the one for your platform:

- `agentctl-linux-amd64`
- `agentctl-linux-arm64`
- `agentctl-darwin-amd64`
- `agentctl-darwin-arm64`

---

## 📄 License

Apache License 2.0 — see [LICENSE](LICENSE).
