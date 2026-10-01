# ADR: how Clawdlinux decides allow / deny / approve

Status: proposed 2026-10-01. Source of truth for docs, deck, site, goal prompt. If another doc disagrees, this wins until changed here.

## Decision
Three layers. Model scores. Rules bound. Humans resolve uncertainty. Everything signed.

1. Invariants (code, tiny, deterministic). Things that must never happen whatever any model says. Credentials never reach agent. Egress only to declared destinations. Personal data never leaves cluster to undeclared destination. Receipts required or workload fails closed. Target: under 15 invariants. Model cannot override.
2. Decision model (learned, scores each action). Takes observed facts plus declared purpose plus history. Returns risk score, per-option weights, reason codes, calibration. Phase A escalate-only: can push to approval, never turn deny into allow. Phase B bounded autonomy: may auto-allow inside invariants when calibrated on approval history above threshold, per action class, with rollback.
3. Human approval. Resolves low-confidence and high-impact. Each decision is a labelled example (approve, reject, edit) in the signed approval dataset.

## Inputs, by trust
- Declared (manifest, human-reviewed in PR): purpose, decision type, allowed data classes, allowed destinations.
- Observed (platform measures): destination, detected data classes (deterministic detectors), tokens, tool, caller identity, history.
- Agent-claimed: self-reported confidence and intent. Logged in receipt. Used only to tighten. Never loosens.

## Why not model-only
Auditor needs reproducible "why". Model gate is itself a model that must be validated. Prompt injection moves model verdicts. So model never owns the floor.

## Why not rules-only
Rule count explodes, edge cases never end. Policy documents are prose. So model compiles policy prose into rule drafts (human reviews diff once) and scores the gray area.

## Receipt must record for every model decision
model id and version, input hash, option set and order, per-option probabilities, threshold used, outcome, layer that decided. Replay with same inputs must reproduce the score (pin version, temperature 0, fixed seed). Air-gap: model runs in cluster (self-hosted small model). No hosted scoring API in the default path.

## Today vs target (verified in code 2026-10-02)
- Threshold evaluator moved from pkg/opa to pkg/rules/threshold. Still
  hardcoded Go thresholds. pkg/opa is a deprecated alias package.
- go.mod now has OPA (v1.19.1). pkg/rules/engine embeds it and evaluates the
  shipped packs (dpdp-in, gdpr-eu) when spec.policyPacks is set. The
  config/policies and pkg/rules/threshold .rego samples are still unevaluated.
- Invariants (pkg/invariants, INV-01 to INV-05) run on every direct-path
  action. See [invariants](invariants.md). INV-05 fails closed when
  `RECEIPTS_REQUIRED=true` and no receipt can be written.
- Receipts (pkg/receipts, cmd/receipt-writer) are opt-in. With receipts on,
  every direct-path decision gets a signed AgentGate-format receipt before the
  action runs. See [receipts](../receipts.md). A scored action carries a
  `model` block. Execution outcome and the runtime-adapter path are not
  receipted.
- Layers combine in pkg/decision.Decide. Strictest wins.
- P7 decision tracing covers direct proposals and stamped human decisions.
  Evaluation spans join successful receipt appends through sequence and entry hash.
  Runtime adapters, execution outcomes, writer internals, and writer trace propagation remain untraced.
  See [decision tracing](../TRACING.md#decision-tracing).
- Human approval (layer 3) works on the direct path. Decisions are
  annotations; the webhook stamps the approver. Each decision gets a signed
  layer "human" receipt and one example in approvals.jsonl. Approve and edit
  re-run invariants and packs first. A human never overrides an invariant.
  See [approvals](../approvals.md). Gap: Argo approval gates are not covered.
- Gap: the runtime-adapter path (reconcileViaRuntime) runs neither invariants
  nor packs. Packs on that path fail closed.
- Input confidence comes from the propose_action tool reply. cluster_health comes from the MCP status reply, default 75 when absent. Platform measures neither.
- "Destructive" = string match on action name.
- pkg/evaluation/scorer.go: keyword heuristic, post hoc, blocks nothing.
- AgentGate: deterministic, fine as invariant layer example.
- Layer 2, phase A exists: a logistic regression decision model
  (pkg/decision/learned) with integer-only inference, escalate-only via
  pkg/decision.ApplyModel, shadow by default, scored after Decide on the
  direct path. Nothing runs until DECISION_MODEL_PATH points at an artifact.
  The shipped artifact is trained on 30 DRAFT synthetic scenarios and is not
  production-validated. Offline evaluation: `agentctl decision eval`. See
  [decision model](decision-model.md). Phase B (bounded autonomy) is not built.

## Build order
1. Measure, stop trusting agent self-report (observed inputs). See
   [policy input](../policy-input.md).
2. Real rule engine (OPA Go lib) for invariants and packs.
3. Receipts for every decision (shared receiptspec). Shipped on the direct
   path, opt-in. See [receipts](../receipts.md).
4. Approval dataset (already in goal prompt phase 4). Shipped on the direct
   path: signed approve, reject, edit receipts plus approvals.jsonl. See
   [approvals](../approvals.md).
5. Decision model phase A (escalate-only), evaluated offline on approval
   dataset first, shadow mode before it can affect anything. Shipped:
   classifier, shadow and escalate modes, model block in receipts, offline
   eval with a baseline, deterministic stdlib trainer. Not done: training on
   real labels, the gate in [decision model](decision-model.md), the LLM
   judge as a second signal. See [decision model](decision-model.md).
6. P7 trace coverage for direct decisions. Shipped: layer spans, receipt joins, privacy tests, and `make trace-coverage`.
  Runtime and cross-process gaps remain. See [decision tracing](../TRACING.md#not-covered).
7. Phase B only after shadow results and calibration are documented.

## Candidate model types (decide by spike, not now)
Small fine-tuned classifier on structured features (cheap, fast, auditable features). Small local LLM judge with typed options (Jev-style, AgentJev-0.6B is Apache-2.0). Start with the classifier. LLM judge as second signal. Both escalate-only at first.

## Open
- Feature set and labels: seeded from approval dataset, but dataset is empty until first PoC. Need synthetic seed from founder-written scenarios (50 to 100) to bootstrap shadow mode. Today: feature spec v1 and 30 engineer-written DRAFT scenarios in tools/decision-train/scenarios.json, to be replaced.
- Who owns threshold changes (customer security team) and how they are versioned.
