# Clawdlinux Design Doc

Status: living document, July 2026. If you read one doc about this project, read this one.

## The one-glance version

Enterprises want to run AI agents on their own Kubernetes clusters. Their security and compliance teams keep saying no, because nobody can answer four questions about an autonomous agent: who is it acting as, what can it reach, what did it cost, and can we prove what it did six months later.

Clawdlinux is a governance plane that answers those questions for any agent workload on Kubernetes. It does not compete with agent runtimes. It wraps them.

The wedge is the combination of two controls in one self-hosted deployment:

1. A signed, hash-chained attestation artifact for every agent run, verifiable offline with a small CLI (`audit-verify`). An auditor can replay what an agent did months later inside an air-gapped cluster.
2. A zero-egress seal at the network boundary. The agent can only reach the FQDNs its manifest declares. Enforced by Cilium, not by prompting the model to behave.

Everything else (gVisor isolation, cost attribution, Argo orchestration, LiteLLM routing) supports that wedge.

## Architecture

Everything in the governance plane runs inside one Kubernetes cluster, which can be fully air-gapped. The operator also runs the tenant controller (namespaces, RBAC, quotas), the offline JWT license validator, and the gVisor RuntimeClass injector. Runtime adapters (`pkg/runtime`) apply the same seal and attestation contract to our AgentWorkload CRD, BYO labeled pods, and external CNCF runtimes. Shared services (Argo, LiteLLM, MinIO, Postgres, Grafana, Browserless) ship as subcharts of one Helm umbrella chart.

Data flow for one run: AgentWorkload applied -> license and OPA policy check -> Cilium egress policy generated from the manifest's declared FQDNs -> Argo DAG executes agent pods (gVisor if labeled) -> every LLM call, tool call, and state transition appended to the audit chain -> cost recorded per workload -> signed attestation artifact written to MinIO -> chain head checkpointed.

Tamper anywhere in that record and `audit-verify` fails the chain. That is the demo moment.

## Why the audit chain is trustworthy

Each event hash commits to the previous one (seq, timestamp, tenant, workload, actor, action, payload hash, prev hash, fixed binary encoding). Rows are HMAC-signed with a key held in a Kubernetes Secret. Storage is append-only with no UPDATE/DELETE grants. The chain head is published periodically to a ConfigMap and optionally mirrored to Sigstore Rekor. Verification is a pure function over bytes, so a third party recomputes the chain offline without trusting our server. Code: `pkg/audit`, CLI: `cmd/audit-verify`.

## Anchor use case: compliance-bound market analysis at a financial institution

Customer assumptions, stated plainly:

- A hedge fund or bank platform team, 5 to 50 engineers, already running Kubernetes 1.27+.
- They are under SEC/FINRA/FCA-style audit obligations. Cloud AI SaaS is banned or heavily gated by infosec.
- They want agents that scrape and analyze market data, summarize filings, or triage alerts. The tech works. Deployment is blocked on governance, not capability.
- They have an approved LLM path: a local vLLM box in the air gap, or a vetted external API behind an egress allowlist.

How they integrate, day one to day thirty:

1. `helm install` the umbrella chart from an OCI artifact we hand them. Air-gapped clusters load images from the same bundle. No registry pulls, no license server callbacks (JWT verified offline).
2. Point LiteLLM at their approved model endpoint. Nothing else in the stack knows or cares which model it is.
3. Bring their agents. Three adoption paths, cheapest first: label existing pods (they get the gVisor seal and audit for free), wrap them in an AgentWorkload CRD (they also get Argo DAG orchestration, budgets, and routing), or run a supported CNCF runtime behind the adapter.
4. Compliance officer gets a standing artifact: for each run, who ran it, what it touched, what it cost, signed and replayable. When the auditor shows up, they hand over a snapshot and the `audit-verify` binary instead of a week of log archaeology.

What they get out of it, in their words: "we can finally say yes to the agent project" (platform lead), "blast radius is declared in a manifest I can review in a PR" (security), "evidence collection went from days to a command" (compliance), "I know what each agent costs per run" (whoever owns the cloud bill).

What we deliberately do not do: build another agent framework, host their data (there is no SaaS in the loop), or ask them to rewrite agents. Governance wraps what they have.

## ACP in one paragraph

Agent Contract Protocol is the same philosophy applied to tool calls. MCP answers what tools exist; ACP answers what execution is allowed for this agent right now. The server consumes MCP `tools/list` and returns a signed execution contract: scoped capabilities, identity binding, credential injection at the proxy (secrets never enter model context), declared ordering, egress and approval policy. Measured side effect: 64.7% to 97.4% fewer setup tokens and one round trip instead of up to 21, across five benchmark scenarios (see agent-contract-protocol/results). Separate repo, separate spec (v0.2 draft), feeds the same audit chain.

## Repo map

Two product repos plus a website, all under github.com/Clawdlinux.

`agentic-operator-core` is the platform. `api/` and `internal/controller/` hold the CRD and reconcilers. `internal/admission/` is the gVisor RuntimeClass injector. `pkg/` has the subsystems: `audit` (hash chain), `runtime` (adapters), `finops`, `opa`, `llm`, `argo`, `multitenancy`, `mcp`. `cmd/` builds the operator, `agentctl`, `audit-verify`, and the license generator. `charts/` is the Helm umbrella with subcharts for Argo, LiteLLM, MinIO, Postgres, Browserless, and observability. `agents/` is the Python LangGraph runtime with A2A. `examples/` has the multi-agent swarm and SRE scenarios. `scripts/demo-booth.sh` is the reproducible demo. `docs/` is numbered 01 through 12 plus RFCs.

`agent-contract-protocol` is the ACP spec (SPEC.md), Go reference server, Python client, adapters, and the benchmark harness with checked-in results.

`clawdlinux-website` is the public site (clawdlinux.org).

## Honest state of the build

Working today: CRD and reconciliation lifecycle, Argo DAG orchestration, Cilium egress policy generation, audit chain and offline verifier, gVisor injector, runtime adapters (AgentWorkload, BYO pods, Argo), LiteLLM routing, per-workload cost tracking, multi-tenancy with quotas, offline JWT licensing, Helm umbrella chart, agentctl with an MCP server mode, Grafana dashboards, the kind-based demo gate, ACP reference server with measured benchmarks.

Not done, do not claim it: webhook admission validation for the CRD, Homebrew tap, air-gapped install smoke test as CI, per-runtime sandbox label guide, multi-cluster identity federation (SPIFFE/SPIRE, RFC-0001, gated on 6 external use cases or 1 paying customer), SOC 2, managed SaaS. Enterprise billing and licensing internals live in a private repo; the OSS tree has boundary READMEs only.

The rule we run by: anything that smells like rebuilding the runtime layer sits behind a validation gate until a real deployment asks for it. See ROADMAP.md.

## Positioning constraints (read before writing public copy)

We sell the attestation and governance plane. Runtimes underneath are supported details. Never make a named runtime the subject of a problem sentence, never claim a competitor lacks something it has, and mention competitors only in a supported-runtimes context. See [the architecture guide](04-architecture.md) and [roadmap](../ROADMAP.md).
