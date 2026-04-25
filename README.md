<p align="center">
  <img src="assets/logo.svg" alt="Agentic Operator" width="90" height="90" />
</p>

<h1 align="center">Agentic Operator</h1>

<p align="center">
  <strong>The only Kubernetes agent platform built for zero-egress, regulated environments.</strong>
</p>

<p align="center">
  One <code>AgentWorkload</code> manifest. Air-gapped by default. Argo DAG orchestration. Per-tenant cost attribution. Zero cloud lock-in.
</p>

<p align="center">
  <!-- Core -->
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-Apache_2.0-3B82F6?style=flat-square" alt="License" /></a>
  <a href="go.mod"><img src="https://img.shields.io/github/go-mod/go-version/Clawdlinux/agentic-operator-core?style=flat-square&color=3B82F6" alt="Go Version" /></a>
  <a href="https://github.com/Clawdlinux/agentic-operator-core/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/Clawdlinux/agentic-operator-core/ci.yml?style=flat-square&label=CI&color=3B82F6" alt="CI" /></a>
  <a href="https://github.com/Clawdlinux/agentic-operator-core/actions/workflows/test-gates.yml"><img src="https://img.shields.io/github/actions/workflow/status/Clawdlinux/agentic-operator-core/test-gates.yml?style=flat-square&label=Tests&color=3B82F6" alt="Test Gates" /></a>
</p>

<p align="center">
  <!-- Ecosystem -->
  <img src="https://img.shields.io/badge/Kubernetes-1.27+-326CE5?style=flat-square&logo=kubernetes&logoColor=white" alt="Kubernetes" />
  <img src="https://img.shields.io/badge/Helm-3.x-0F1689?style=flat-square&logo=helm&logoColor=white" alt="Helm" />
  <img src="https://img.shields.io/badge/Argo-Workflows-EF7B4D?style=flat-square&logo=argo&logoColor=white" alt="Argo Workflows" />
  <img src="https://img.shields.io/badge/Cilium-FQDN_Egress-F8C517?style=flat-square&logo=cilium&logoColor=black" alt="Cilium" />
  <img src="https://img.shields.io/badge/LiteLLM-Model_Routing-6366f1?style=flat-square" alt="LiteLLM" />
  <img src="https://img.shields.io/badge/OpenMeter-Cost_Attribution-ec4899?style=flat-square" alt="OpenMeter" />
</p>

<p align="center">
  <!-- Actions -->
  <a href="docs/01-quickstart.md"><img src="https://img.shields.io/badge/⚡_Quick_Start-5_Minutes-3B82F6?style=for-the-badge" alt="Quick Start" /></a>
  <a href="https://clawdlinux.org"><img src="https://img.shields.io/badge/🌐_Website-clawdlinux.org-1e293b?style=for-the-badge" alt="Website" /></a>
  <a href="https://discord.gg/2yJsjhPe"><img src="https://img.shields.io/badge/💬_Join-Discord-5865F2?style=for-the-badge&logo=discord&logoColor=white" alt="Discord" /></a>
  <a href="docs/04-architecture.md"><img src="https://img.shields.io/badge/📐_Architecture-Deep_Dive-1e293b?style=for-the-badge" alt="Architecture" /></a>
  <a href="CONTRIBUTING.md"><img src="https://img.shields.io/badge/🤝_Contribute-Guidelines-1e293b?style=for-the-badge" alt="Contribute" /></a>
</p>

---

## 🎬 Demo

https://github.com/Clawdlinux/agentic-operator-core/raw/main/assets/agentic-operator-demo.mp4

> 24-second walkthrough: problem → manifest → `kubectl apply` → live workload status.

---

## Why Agentic Operator?

kagent (Solo.io, CNCF Sandbox) validates this market completely. When Google, Microsoft, IBM, and Red Hat contribute to a Kubernetes agent runtime, the category is real.

But here's what kagent **structurally cannot do**:

| Capability | Agentic Operator | kagent (Solo.io) |
|---|:---:|:---:|
| **Air-gapped / zero-egress deployment** | ✅ | ❌ |
| **Outcome-based billing per workload** | ✅ OpenMeter | ❌ |
| **Argo DAG orchestration** | ✅ | ❌ |
| **Per-tenant cost isolation** | ✅ | ❌ |
| **JWT offline licensing** | ✅ | ❌ |
| Kubernetes-native operator | ✅ | ✅ |
| OTel observability | ✅ Langfuse + OTel | ✅ |
| RBAC / identity | ✅ Cilium + RBAC | ✅ mTLS + OIDC |
| MCP support | ✅ | ✅ |
| Multi-framework support | ✅ LangGraph (Python) | ✅ LangChain, CrewAI, ADK |

> **kagent is excellent for cloud-connected teams. We are the only option for environments where data cannot leave the network — air-gapped, FedRAMP, HIPAA-constrained, and sovereign cloud.** Those buyers have no other choice.

Platform teams running AI agents on Kubernetes today face a painful reality: each agent framework expects its own runtime, its own secrets, its own network rules. You end up with a sprawl of bespoke Deployments, no cost visibility, and no guardrails.

**Agentic Operator fixes this.** One CRD, one controller, full-stack isolation:

| Problem | Agentic Operator |
|---------|-----------------|
| Agent sprawl across namespaces | Single `AgentWorkload` CRD per agent |
| No network boundaries | Cilium FQDN egress policies auto-applied |
| Invisible costs | Per-workload token metering + cost attribution |
| Manual DAG wiring | Argo Workflows orchestrates agent steps |
| Vendor lock-in | Any LLM via LiteLLM proxy routing |
| Cloud-only runtimes | Full air-gapped, offline-first deployment |

---

## Demo

```
$ kubectl apply -f agentworkload.yaml
agentworkload.agentic.clawdlinux.org/research-run created

$ kubectl get agentworkload research-run -w
NAME           PHASE       AGE
research-run   Pending     0s
research-run   Isolating   2s    # namespace + cilium policy applied
research-run   Running     5s    # argo workflow launched
research-run   Completed   47s   # artifacts retained in minio

$ kubectl logs -n aw-research-run agent-pod --tail=5
[agent] analyzing Q1 2026 technology trends...
[agent] sources: arxiv, github trending, HN front page
[agent] cost: $0.0023 (gpt-4o-mini) | tokens: 1,847 in / 892 out
[agent] output written to minio://research-run/report.md
[agent] run complete — 42s wall time
```

---

## Quick Start

**Option A — One command (requires kind + helm):**
```bash
curl -sSL https://raw.githubusercontent.com/Clawdlinux/agentic-operator-core/main/scripts/install.sh | bash
```

**Option B — Step by step:**
```bash
git clone https://github.com/Clawdlinux/agentic-operator-core
cd agentic-operator-core

# Create local cluster
kind create cluster --name agentic-operator

# Install CRD + operator
kubectl apply -f config/crd/agentworkload_crd.yaml
helm dependency build ./charts
helm upgrade --install agentic-operator ./charts \
  --namespace agentic-system --create-namespace

# Deploy your first agent
kubectl apply -f config/agentworkload_example.yaml
kubectl -n agentic-system get agentworkloads -w
```

**Option C — GitHub Codespaces (zero local setup):**

[![Open in GitHub Codespaces](https://github.com/codespaces/badge.svg)](https://codespaces.new/Clawdlinux/agentic-operator-core?devcontainer_path=.devcontainer/devcontainer.json)

---

## Architecture

```mermaid
flowchart LR
    A[AgentWorkload CRD] --> B[Agentic Operator Controller]
    B --> C[Argo Workflows DAG]
    C --> D[Agent Runtime Pods]
    D --> E[LiteLLM Proxy]
    D --> F[Browserless Pool]
    D --> G[MinIO Artifacts]
    B --> H[Policy & Isolation Layer]
    H --> I[Cilium FQDN Egress]
    H --> J[OPA Admission]
    B --> K[Metering & SLA]
```

---

## What's Included

| Component | Description |
|-----------|-------------|
| **AgentWorkload CRD** | Declarative spec for agent objective, model, quotas, egress rules |
| **Controller** | Reconciles workloads → namespaces, network policies, workflows, artifacts |
| **Argo Integration** | Agent steps execute as DAG nodes with retries and timeouts |
| **Cilium Policies** | FQDN-based egress lock-down auto-generated per workload |
| **LiteLLM Routing** | Cost-aware model selection across providers (OpenAI, Anthropic, Cloudflare) |
| **MinIO Artifacts** | Per-workload bucket for prompts, logs, outputs, audit trails |
| **Multi-tenancy** | Namespace isolation with quota enforcement per tenant |
| **Cost Attribution** | Per-workload usage metering and cost-attribution hooks for chargeback reporting |
| **Python Agent Runtime** | Batteries-included agent framework with tool integrations |

---

## Product Editions

| | Community | Enterprise |
|---|---|---|
| **License** | Apache 2.0 — free forever | Contact for pricing |
| **Deployment** | Self-managed | Managed + self-managed |
| **Air-gapped support** | ✅ | ✅ |
| **AgentWorkload CRD** | ✅ | ✅ |
| **Argo DAG orchestration** | ✅ | ✅ |
| **Cilium egress policies** | ✅ | ✅ |
| **Cost attribution hooks** | ✅ | ✅ |
| **Managed upgrades** | — | ✅ |
| **Dedicated control plane** | — | ✅ |
| **Private registry & SSO** | — | ✅ |
| **SLA + incident response** | — | ✅ |
| **FedRAMP / HIPAA advisory** | — | ✅ |

Enterprise inquiries: [shreyanshsancheti09@gmail.com](mailto:shreyanshsancheti09@gmail.com?subject=Enterprise%20Inquiry)

---

## Repository Layout

```
cmd/                    Operator entrypoint
internal/controller/    Reconciliation logic
api/v1alpha1/           CRD API types and schema
agents/                 Python agent runtime
charts/                 Helm umbrella chart
config/                 CRD, RBAC, sample manifests
docs/                   Documentation
pkg/                    Shared packages (billing, license, autoscaling, routing)
tests/                  Integration + E2E test suites
assets/                 Branding assets (logo, etc.)
```

---

## Documentation

| Doc | Description |
|-----|-------------|
| [Quick Start](docs/01-quickstart.md) | 5-minute setup guide |
| [Installation](docs/02-installation.md) | Production deployment options |
| [Configuration](docs/03-configuration.md) | CRD fields, Helm values, tuning |
| [Architecture](docs/04-architecture.md) | System design deep dive |
| [Multi-tenancy](docs/05-multi-tenancy.md) | Tenant isolation and quota enforcement |
| [Cost Management](docs/06-cost-management.md) | Per-workload billing and chargeback |
| [Security](docs/07-security.md) | Cilium, OPA, RBAC, and egress hardening |
| [Troubleshooting](docs/10-troubleshooting.md) | Common issues and fixes |

---

## Open Source Boundary

This repository is the **open-source core**. It contains everything needed to run agent workloads on Kubernetes, including full air-gapped support.

The [private companion](https://github.com/Clawdlinux/agentic-operator-private) adds enterprise features built on top of the core's cost-attribution primitives:
- License validation and trial enforcement
- External billing system integrations (e.g. OpenMeter, Stripe, internal chargebacks)
- Production DOKS deployment overlays
- FedRAMP / HIPAA compliance overlays

---

## Contributing

We welcome contributions! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

```bash
# Fork, clone, create a branch
git checkout -b feat/my-improvement

# Run tests
make test

# Submit a PR
```

---

## Roadmap

See [ROADMAP.md](ROADMAP.md) for the public roadmap and quarterly milestones.

---

## Community

- **Discord** — [Join our Discord](https://discord.gg/2yJsjhPe) for questions, discussions, and design partner conversations
- **Issues** — [Report bugs or request features](https://github.com/Clawdlinux/agentic-operator-core/issues)
- **Releases** — [Subscribe to releases](https://github.com/Clawdlinux/agentic-operator-core/releases) for changelog updates

---

## License

Apache License 2.0 — See [LICENSE](LICENSE).
