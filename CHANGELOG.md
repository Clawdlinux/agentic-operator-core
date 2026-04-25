# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.2.0] - 2026-04-25

Production-ready agent orchestration release with multi-agent demos, pluggable workflows, CLI onboarding, and landing page overhaul.

### Added
- **Multi-Agent Swarm Example** — A2A agent discovery, OPA budget enforcement, autoApproveThreshold gating, Docker Compose with ollama local overlay (#60)
- **SRE Incident Response Example** — collaborationMode: delegation, 3-tier cost-aware model routing, adversarial tone remediator, hierarchical memory (#62)
- **Pluggable Workflow Registry** — `@register_workflow` decorator, auto-discovery from built-in + custom dirs, `WORKFLOW_NAME` env var selection (#64)
- **Code Review Workflow** — Fan-out security/performance/style analysis agents
- **Doc Processor Workflow** — Parallel entity extraction + summarization
- **In-Memory Cost Tracker** — MemoryCostReporter with Prometheus metrics (`agentic_workload_cost_usd`, `agentic_workload_tokens_total`), pre-loaded pricing for OpenAI/Anthropic/Azure/Ollama
- **agentctl init** — Interactive cluster onboarding wizard
- **agentctl approve** — Resume PendingApproval workloads via annotation patch + Argo resume
- **agentctl workflows** — List registered workflows
- **agentctl status** — Cluster health overview (#66)
- **WorkflowName CRD field** — `spec.workflowName` on AgentWorkload for workflow selection
- **"Why Now" Positioning Section** — 4 pillars: Air-Gapped, Agent FinOps, K8s-Native CRDs, Pluggable Workflows (#68)
- **GitHub social preview card** — 1280×640 Open Graph image
- **GitHub Sponsors** — `.github/FUNDING.yml` for sponsorship

### Changed
- **Landing page simplified** — 13 sections → 7, nav reduced from 8+3 → 5+1 (#69)
- **Contact form** — Google Apps Script webhook with `mode: 'no-cors'` (#70)
- **Repo hygiene** — Removed 60MB pptx, internal dev logs, completion reports (#67)
- **LiteLLM healthcheck** — Use `/health/liveliness` (no auth) instead of `/health`

### Fixed
- LiteLLM healthcheck 401 in Docker Compose environments
- OSS/Private boundary CI failure for memory_reporter.go
- gofmt formatting on memory_reporter.go + deepcopy regeneration
- Merge conflicts in multi-agent-swarm files after squash merge
## [0.1.1] - 2026-03-31

Customer-ready release of Agentic Operator — Kubernetes-native multi-agent orchestration framework.

### Added
- **AgentPersona CRD** — First-class Kubernetes resource for agent identity (Role, Tone, MemoryScope, SystemPrompt, ToolProfile)
- **agentctl CLI** — Complete agent lifecycle management with 6 subcommands (get, describe, logs, cost, apply, version) and table/JSON/YAML output
- **Evaluation Framework** — Built-in agent quality metrics (accuracy, consistency, cost, latency) with scorer interface for custom evals
- **MCP Protocol Client** — Native Kubernetes-to-tool integration with MCP support (97M monthly SDK downloads)
- **AgentCard CRD** — A2A-compatible agent discovery model for multi-tenant K8s environments
- **Research Swarm Quickstart** — 4-command Docker Compose demo (research → write → edit pipeline)
- **FinOps Package** — `enterprise/billing` and `enterprise/licensing` for cost tracking and enterprise policies
- **Resilience Package** — Circuit breakers, retry policies, deadline management for reliability
- **Metrics & Observability** — Per-agent cost, latency, error rate tracking; Prometheus-compatible metrics
- **Multitenancy Package** — Tenant isolation, RBAC, quota enforcement
- **Autoscaling Package** — Agent pool scaling based on workload demands
- Makefile with canonical `test`, `validate`, `lint`, `build`, `helm-lint` targets
- CI test gate workflow (`test-gates.yml`) — runs on every PR
- Landing quality gate workflow (`landing-quality-gate.yml`)
- SECURITY.md vulnerability disclosure policy
- Webhook admission infrastructure (`webhook.yaml`) with cert-manager integration
- Helm `lookup` for MinIO/PostgreSQL secrets — preserves credentials across upgrades
- `reportlab` dependency for PDF report generation
- `existingSecret` pattern for Cloudflare Workers AI token
- `agents/requirements-test.txt` for reproducible Python test environments

### Changed
- RBAC: fixed API group (`agentic.io` → `agentic.clawdlinux.org`), least-privilege verbs
- CRD: HTTPS-only MCP endpoint enforcement (`^https://`)
- Webhook validation rejects non-HTTPS MCP endpoints
- `mustToJSON` (panic) replaced with `toJSON` (error return)
- License secret template: added `LICENSE_JWT` and `LICENSE_PUBLIC_KEY_B64` canonical keys
- MinIO `rootUser` default changed from `minioadmin` to empty (auto-generated)
- `values.schema.json` relaxed password `minLength` for auto-generation

### Fixed
- 13 staticcheck warnings resolved (deprecated `ioutil`, unused fields/funcs, nil checks)
- RBAC wildcard `resources: ["*"]` replaced with explicit resources
- Removed dangerous `clusterroles`/`clusterrolebindings` write access
- Default credentials removed from `values.yaml`

### Security
- HTTPS-only enforcement for all MCP server endpoints
- Webhook TLS via cert-manager certificates
- Secret preservation across Helm upgrades via `lookup`
- Repository secret scanning in CI

- Removed dangerous `clusterroles`/`clusterrolebindings` write access
- Default credentials removed from `values.yaml`

### Security
- HTTPS-only enforcement for all MCP server endpoints
- Webhook TLS via cert-manager certificates
- Secret preservation across Helm upgrades via `lookup`
- Repository secret scanning in CI
