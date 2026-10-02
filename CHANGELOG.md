# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

Decision architecture series. The direct action path now decides in layers:
invariants, policy packs, a threshold evaluator, an escalate-only decision
model in shadow, and human approval. Each decision can get a signed receipt
before the action runs. `scripts/demo-claims.sh` proves this on kind. The
runtime-adapter path, Argo approval gates, and execution outcomes are not
covered yet. See [decision architecture](docs/architecture/decision-architecture.md).

### Fixed
- Status-only updates no longer re-trigger Reconcile. Before, each status write re-ran the direct-path action in a hot loop.
- The controller no longer logs MCP status, proposal, or execution values. They can carry personal data or credentials.

### Fixed (review findings)
- An incomplete data scan (budget, depth or size limit) now denies through INV-06 instead of failing open.
- Agent-claimed inputs can only raise model risk. The model scores with and without claims and takes the higher value.
- The direct path reserves execution with a status update before it runs an allowed action, so a stale reconcile cannot run after the spec tightens.
- The remote receipt writer client verifies each receipt under a pinned writer key.
- `agentctl dataset verify` requires `--trust-root` (or `--allow-embedded-trust` for a consistency-only check). `agentctl decision eval` marks data verified only with a pinned `--trust-root`. The webhook rejects changes to `declaredIntent`, `policyPacks`, `opaPolicy`, `decisionModel` and `approvalCapture` while an approval is pending.
- `agentctl receipts verify` and `agentctl dataset verify` need a signed manifest by default. `--allow-prefix` is the weaker mode.
- Receipts bind the executed payload by digest (`payload_sha256`).
- The approval stamp carries an HMAC, and a consumed approval cannot be replayed.
- `agentctl` approval patches carry the resourceVersion and fail on conflict.
- A configured model that fails to load is treated as unavailable. Escalate mode then requires approval.
- Approver identities are stored as digests in receipts.
- MCP error bodies and invalid values are no longer logged.

### Added
- Added `scripts/demo-claims.sh`, a no-API-key claims demo on a fresh kind cluster with a mock MCP server. It runs with a pinned writer key and an approval stamp key, and proves the review fixes: wrong pin, removed manifest, approval replay, identity digests, and INV-06. Two fresh runs passed in 252s and 297s. It builds a demo-only image from host binaries until the receiptspec dependency is tagged.
- Added P7 decision spans on direct proposals and stamped human approve, reject, and edit decisions.
	Successful receipt appends bind spans to writer-returned sequence and entry hash.
	Added privacy canary tests, no-op checks, write-ahead assertions, and `make trace-coverage` with an 11-path matrix.
	Runtime adapters, execution outcomes, receipt-writer internals, and writer trace propagation remain untraced.
	See [decision tracing](docs/TRACING.md#decision-tracing).
- Added a working booth prepare/present flow with real provider and cost proof.
- Added `agentctl doctor sandbox` to verify RuntimeClass and ready-node sandbox evidence before workload deployment.
- Added optional `spec.declaredIntent` on `AgentWorkload` and observed decision inputs. The legacy action path now denies or escalates actions that send undeclared data classes or call undeclared hosts. Agent-claimed confidence and cluster health can only tighten. New packages: `pkg/dataclass`, `pkg/decision`, `pkg/decision/input`, `pkg/decision/observe`. See `docs/policy-input.md`.
- Added invariants (`pkg/invariants`, INV-01 to INV-06; INV-06 denies a payload the data class scan could not fully read), an embedded OPA rule engine (`pkg/rules/engine`, OPA v1.19.1), and opt-in policy packs `dpdp-in@v0.1.0` and `gdpr-eu@v0.1.0` via `spec.policyPacks`. Packs only add deny or require approval. Packs are engineering controls, not legal advice. The `credential` data class is now detected and always denied (INV-01), even without `declaredIntent`. Direct action path only. See `docs/policy-packs.md`.
- Added opt-in signed decision receipts (`pkg/receipts`, `cmd/receipt-writer`). Each direct-path decision gets an AgentGate-format Ed25519 receipt before the action runs. `RECEIPTS_REQUIRED=true` makes INV-05 deny when no receipt can be written. Added `agentctl receipts export` and `agentctl receipts verify`, and a `global.receipts` chart block. Records hold hashes and class names, not raw params or matched data. Execution outcome and the runtime-adapter path are not receipted yet. Uses the public `pkg/receiptspec` from `github.com/Clawdlinux/agentgate` v0.1.4. See `docs/receipts.md`.
- Added human approvals on the direct action path (`pkg/approval`, `pkg/dataset`). A held action is stored in `status.pendingApproval`. Humans decide with `clawdlinux.org/approval-decision` (approve, reject, edit). The webhook stamps the approver; the controller refuses unstamped decisions. Each decision gets a layer `human` receipt and an example in `approvals.jsonl` on the receipt-writer (`POST|GET /v1/approvals`). Edited and approved actions re-run invariants and packs first. Added `spec.approvalCapture.content` (none, redacted, full; default redacted), phase `Rejected`, `agentctl edit-approve`, `agentctl approve --reason`, and `agentctl dataset export|verify`. See `docs/approvals.md`.
- Added the decision model, phase A (`pkg/decision` features and `ApplyModel`, `pkg/decision/learned`, `pkg/decision/baseline`, `pkg/decision/eval`). Escalate-only: it can move allow to require_approval and never loosens. `spec.decisionModel.mode` (off, shadow, escalate; default shadow) and a stricter-only `thresholdMicro`. The operator loads an artifact from `DECISION_MODEL_PATH`; without one nothing is scored. Scored decisions carry a receipt `model` block with integer micro-unit probabilities. Added `agentctl decision eval`, `make decision-train` (stdlib Python, byte-identical reruns), and `global.decisionModel` in the chart. The shipped artifact is trained on 30 DRAFT synthetic scenarios and is not production-validated. Approval examples gain an optional `source` field; `synthetic` rows never verify. See `docs/architecture/decision-model.md`.

### Changed
- Moved the threshold evaluator from `pkg/opa` to `pkg/rules/threshold`. `pkg/opa` remains as a deprecated alias package.
- `agentctl` table output now uses tablewriter v1 (box-drawn borders). OPA v1.19.1 requires it.
- An unknown or unenforceable `spec.policyPacks` entry fails the workload closed with condition `PolicyPackInvalid`. The validating webhook also rejects unknown packs.
- The AgentWorkload webhooks now register handlers. Before, `SetupWebhookWithManager` registered none, so enabling `agentWorkloadEnabled` sent requests to missing paths.
- A `PendingApproval` workload with a stored pending action waits for a human decision instead of re-proposing every hour.
- Docs: removed claims not backed by code. Added the decision architecture ADR (`docs/architecture/decision-architecture.md`). Rego files are labelled samples; the action path is a Go threshold evaluator.
- Changed the internal Go `CostReporter.RecordUsage` and `Provider.CallModel` interfaces to carry operation IDs for idempotency. This breaks out-of-tree implementations.
- Added the optional `litellm.anthropicKey` chart value. Use `litellm.existingSecret` for booth and production deployments.

## [0.2.0] - 2026-04-25

Agent orchestration release with multi-agent examples, pluggable workflows, CLI onboarding, and landing page updates.

### Added
- **Multi-Agent Swarm Example** — A2A agent discovery, persona tool checks, approval-threshold inputs, and Docker Compose with an Ollama overlay (#60)
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

Initial Kubernetes-native multi-agent orchestration feature release.

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
