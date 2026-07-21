# Clawdlinux Design Doc

Status: living document, July 2026. If you read one doc about this project, read this one.

## The one-glance version

Enterprises want to run AI agents on their own Kubernetes clusters. Their security and compliance teams keep saying no, because nobody can answer four questions about an autonomous agent: who is it acting as, what can it reach, what did it cost, and can we prove what it did six months later.

Clawdlinux is building a governance plane around agent workloads on Kubernetes. It does not compete with agent runtimes. It wraps them.

The target contract combines two controls in one self-hosted deployment:

1. Same-run signed evidence that links identity, policy, approval, action, cost, and outcome.
2. A declared egress boundary enforced by the customer's Kubernetes networking stack.

Everything else (gVisor isolation, cost attribution, Argo orchestration, LiteLLM routing) supports that wedge.

Today, the repository ships the audit hash-chain and JSONL verifier as separate primitives. It also generates network-policy objects and admission mutations. Automatic same-run capture, durable audit storage, production key custody, and packet-enforcement proof are not connected end to end.

## Architecture

The governance components run inside the customer's Kubernetes cluster. The repository includes tenant controls, offline JWT validation, admission mutation, runtime adapters, and optional shared-service subcharts.

Current data flow: AgentWorkload applied -> license and budget checks -> runtime selected through `pkg/runtime.Registry` -> workload status and usage updated. Admission and network-policy components apply through Kubernetes configuration paths.

The audit package is not called by this reconciliation path. The booth separately verifies a prior-run JSONL fixture and labels it accordingly.

## Audit primitive and limits

Each entry hash commits to its sequence, timestamp, context, payload hash, and previous hash. Rows use HMAC-SHA256 with a caller-supplied key. `audit-verify` can verify JSONL entries offline.

The only repository backend is in-memory. ClickHouse reading, Kubernetes key loading, automatic capture, restricted durable storage, and external checkpoint publishing are not implemented. A shared-key holder can rewrite and re-sign history. Tail truncation can pass without a trusted expected head.

## Anchor use case: compliance-bound market analysis at a financial institution

Customer assumptions, stated plainly:

- A hedge fund or bank platform team, 5 to 50 engineers, already running Kubernetes 1.27+.
- They are under SEC/FINRA/FCA-style audit obligations. Cloud AI SaaS is banned or heavily gated by infosec.
- They want agents that scrape and analyze market data, summarize filings, or triage alerts. The tech works. Deployment is blocked on governance, not capability.
- They have an approved LLM path: a local vLLM box in the air gap, or a vetted external API behind an egress allowlist.

How they integrate, day one to day thirty:

1. `helm install` the umbrella chart from an OCI artifact we hand them. Air-gapped clusters load images from the same bundle. No registry pulls, no license server callbacks (JWT verified offline).
2. Point LiteLLM at their approved model endpoint. Nothing else in the stack knows or cares which model it is.
3. Bring one bounded agent workload. Use a registered runtime adapter or opt labeled pods into supported admission controls.
4. Map identity, policy, approval, storage, keys, and evidence retention to the customer's environment.

The design-partner goal is measurable production-review evidence for one workload. It is not a blanket compliance claim.

What we deliberately do not do: build another agent framework, host their data (there is no SaaS in the loop), or ask them to rewrite agents. Governance wraps what they have.

## Agent Native Format in one paragraph

Agent Native Format (ANF) is the sibling repo. Its headline is a token-minimal view format. It translates system state into far fewer tokens for agents (see FORMAT.md). It also ships an execution runtime that applies the same governance philosophy to tool calls. MCP answers what tools exist. The runtime answers what execution is allowed for this agent right now. The server consumes MCP `tools/list` and returns a signed execution contract: scoped capabilities, identity binding, credential injection at the proxy (secrets never enter model context), declared ordering, egress and approval policy. Measured side effect: 64.7% to 97.4% fewer setup tokens and one round trip instead of up to 21, across five benchmark scenarios (see agent-native-format/results). Separate repo, separate spec, feeds the same audit chain.

## Repo map

Two product repos plus a website, all under github.com/Clawdlinux.

`agentic-operator-core` is the platform. `api/` and `internal/controller/` hold the CRD and reconcilers. `internal/admission/` is the gVisor RuntimeClass injector. `pkg/` has the subsystems: `audit` (hash chain), `runtime` (adapters), `finops`, `opa`, `llm`, `argo`, `multitenancy`, `mcp`. `cmd/` builds the operator, `agentctl`, `audit-verify`, and the license generator. `charts/` is the Helm umbrella with subcharts for Argo, LiteLLM, MinIO, Postgres, Browserless, and observability. `agents/` is the Python LangGraph runtime with A2A. `examples/` has the multi-agent swarm and SRE scenarios. `scripts/demo-booth.sh` is the reproducible demo. `docs/` is numbered 01 through 12 plus RFCs.

`agent-native-format` is the ANF format spec (FORMAT.md) plus an execution runtime: protocol spec (SPEC.md), Go reference server, Python client, adapters, and the benchmark harness with checked-in results.

`clawdlinux-website` is the public site (clawdlinux.org).

## Honest state of the build

Working today: CRD lifecycle, Argo DAG orchestration, network-policy generation, audit primitives, gVisor mutation, runtime adapters, model routing, cost interfaces, quotas, offline JWT validation, Helm packaging, MCP workload tools, dashboards, and the kind-based demo.

Not done, do not claim it: same-run audit capture, durable audit storage, external checkpoints, universal tool mediation, direct MCP approval continuation, actor identity propagation, enforcing-CNI proof, air-gap installation CI, SOC 2, or managed SaaS.

The rule we run by: anything that smells like rebuilding the runtime layer sits behind a validation gate until a real deployment asks for it. See ROADMAP.md.

## Positioning constraints (read before writing public copy)

We sell the attestation and governance plane. Runtimes underneath are supported details. Never make a named runtime the subject of a problem sentence, never claim a competitor lacks something it has, and mention competitors only in a supported-runtimes context. See [the architecture guide](04-architecture.md) and [roadmap](../ROADMAP.md).
