# Decision model (layer 2, phase A)

Status: shipped 2026-10-02, escalate-only, shadow by default, NOT
production-validated. The shipped artifact is trained on synthetic DRAFT
scenarios. Read [Honest status](#honest-status) before quoting any number.

See the [decision architecture ADR](decision-architecture.md) for why the
model sits between invariants and humans.

[Decision tracing](../TRACING.md#decision-tracing) records model identifier, version, and integer risk only when a loaded model runs.

## What it is

A logistic regression over a small, fixed feature vector. It returns the
risk that a human would reject or edit the action. Not an LLM. It runs in the
operator process. No network call, no hosted scoring API.

| Package | Role |
|---|---|
| `pkg/decision` | `Features`, `Score`, `Scorer`, `ApplyModel`, `EffectiveThreshold` |
| `pkg/decision/learned` | Loads the JSON artifact, integer-only inference |
| `pkg/decision/baseline` | Hand-weighted control scorer for evaluation |
| `pkg/decision/eval` | Offline evaluation against approvals.jsonl |
| `tools/decision-train` | Stdlib Python seed and trainer |

## Features (spec v1)

`decision.FeatureSpecVersion = "v1"`. All values are small integers. The
same `FeatureInput` maps from a live decision and from a dataset example, so
training and serving see the same values. `tools/decision-train/features.py`
mirrors `pkg/decision/features.go`. A test recomputes the feature matrix hash
of the shipped dataset to catch drift.

| Feature | Value |
|---|---|
| `dc_personal_count` | distinct personal classes observed (email, phone, aadhaar, pan, iban, card), cap 6 |
| `dc_credential` | credential class observed |
| `dc_undeclared_count` | observed classes outside the declared list, cap 6, only with a manifest |
| `personal_to_undeclared` | personal data and a destination outside the declared list |
| `dest_present`, `dest_declared` | destination host observed, and in the declared list |
| `declared_present` | decision type or an allow list is declared |
| `decision_automated` | declared decision type is `automated` |
| `action_destructive` | an action name token is delete, remove, purge, drop, reset, cleanup, clear, destroy, truncate, wipe |
| `prior_actions_bucket` | prior actions 0, 1-9, 10-99, 100+ as 0..3 |
| `prior_denied` | prior denied count, cap 10 |
| `conf_absent`, `conf_low`, `conf_mid` | claimed confidence absent, below 0.5, 0.5 to below 0.8 |
| `health_absent`, `health_low`, `health_mid` | claimed cluster health absent, below 50, 50 to below 80 |
| `pack_any`, `pack_dpdp_in`, `pack_gdpr_eu` | policy packs set |
| `esc_invariant`, `esc_rules`, `esc_threshold`, `esc_model` | layer of a non-allow outcome |

Declared purpose and agent intent are not features. Dataset examples do not
carry them. Any change to names, order, buckets, or keywords needs a new spec
version. An artifact with another spec version or feature order is refused.

## Artifact

`model.json`, printable ASCII, no floats:

- `schema_version`, `model_id`, `version`, `model_type: logistic_regression`
- `feature_spec_version`, `features` (names in order)
- `weights`, `bias`: decimal strings, at most 6 fractional digits
- `threshold_micro`: escalation threshold, 1 to 1000000
- `calibration`: holdout reliability bins and ECE, integers
- `trained_on`: `data_source` (synthetic, human, mixed), dataset sha256, counts
- `training`: method and hyperparameters as strings
- `artifact_sha256`: SHA-256 of the artifact as compact JSON, sorted keys,
  this field removed

A hash mismatch, spec mismatch, bad decimal, unknown field, or unreadable
file means the configured model is unavailable. The operator logs the error and
keeps an explicit unavailable state. It is handled like a scorer error:
workloads in `escalate` mode require approval for actions the other layers
allow, `shadow` only logs, `off` is unchanged. No `DECISION_MODEL_PATH` at all
still means no model.

## Determinism and replay

- Training: fixed seed, fixed iterations, holdout chosen by
  `sha256(example_id)` mod 5. `make decision-train` trains twice and fails
  unless every output file is byte-identical.
- Inference: weights become int64 micro-units. The logit is an integer sum.
  `SigmoidMicro` computes `exp` with a fixed-point Taylor series and repeated
  multiplication, integers only. No float, no `math.Exp`, so the result does
  not depend on CPU or libm. `golden.json` holds Python-computed features,
  logits, and risks. A Go test checks the operator matches them exactly.
- Replay: same input, same action name, same packs, same artifact give the
  same features, the same `feature_hash`, and the same `risk_micro`.

## Receipt model block

Whenever a model scores an action (shadow or escalate), the decision record
gets a `model` block. See [receipts](../receipts.md#model-block). It holds the
model id and version, artifact sha256, feature spec and feature hash, option
set and order, per-option micro probabilities, risk, threshold used,
calibration note, top reason codes, the outcome before and after the model,
the claimed and claim-independent risks, and the scorer error if any. Model
block schema version is 2.

## Escalate-only guarantee

A model can never turn a deny or a require_approval into an allow. Enforced at:

1. `decision.ApplyModel` is the only function that lets a score change an
   outcome. It returns the input unchanged unless mode is `escalate` and the
   input outcome is `allow`. Then the only output is `require_approval`
   (risk at or above threshold, or a scorer error) or the input.
   `TestApplyModelNeverLoosens` runs every outcome, mode, risk, threshold, and
   error combination and fails on any looser result.
2. The controller calls the model after `decision.Decide`, so invariants,
   packs, and the threshold have already decided. The model sees their
   outcome and cannot replace it.
3. `decision.EffectiveThreshold` only accepts an override that is lower than
   the artifact threshold. The webhook rejects a looser `thresholdMicro` when
   an artifact is loaded. The controller ignores it either way.
4. Scorer error: escalate mode escalates, shadow mode records the error.
   Neither path allows.
5. A model-escalated action goes to the normal approval flow. Human approve
   and edit re-run invariants and packs, not the model.
6. Agent claims only tighten. `decision.ScoreClaimSafe` scores the action
   twice: with the claimed confidence and cluster health, and with both
   removed (`conf_absent`, `health_absent`). Removing claims is the
   claim-independent baseline: the score an agent gets by claiming nothing.
   The risk used is the higher of the two. A mid confidence or healthy
   cluster claim can no longer pull the risk under the threshold.
   `TestClaimsOnlyTighten` checks every confidence and health bucket.
   Both risks go in the model block as `claimed_risk_micro` and
   `baseline_risk_micro`, with `baseline_feature_hash`. The artifact and
   `golden.json` are unchanged: the combination happens above the scorer.
   Offline metrics in `metrics.json` score claimed features only, so they
   can understate the live escalation rate.

## Configuration

| Where | Setting |
|---|---|
| Operator env | `DECISION_MODEL_PATH`, path to `model.json`. Unset: no model |
| Workload | `spec.decisionModel.mode`: `off`, `shadow` (default), `escalate` |
| Workload | `spec.decisionModel.thresholdMicro`: stricter override only |
| Chart | `global.decisionModel.existingConfigMap` and `key` mount the artifact |

Shadow is the default mode, but nothing runs until an artifact is mounted.
Without `DECISION_MODEL_PATH` the operator behaves exactly as before.

```sh
kubectl -n agentic-system create configmap decision-model \
  --from-file=model.json=tools/decision-train/out/model.json
helm upgrade clawdlinux charts --reuse-values \
  --set global.decisionModel.existingConfigMap=decision-model
```

## Training and evaluation

```sh
make decision-train
agentctl decision eval --dataset export/ --model model.json --baseline
agentctl decision eval --dataset tools/decision-train/out/approvals.jsonl \
  --model tools/decision-train/out/model.json --allow-unverified --baseline
```

`eval` verifies an export dir like `agentctl dataset verify` and refuses
unverified data unless `--allow-unverified`. It prints a WARNING header when
any row is synthetic or there are fewer than 200 human labels. Positive means
a human rejected or edited. Recall is the share of those the model escalates.
Extra review is the share of human approvals the model escalates.

### Shipped artifact on its own synthetic data

All 360 rows synthetic, 0 human labels, unverified. These numbers measure fit
to rules an engineer wrote. They say nothing about real reviewers.

| Scorer, threshold 0.5 | Recall | Extra review | Precision | ECE |
|---|---|---|---|---|
| `clawdlinux-escalate-logreg` 0.1.0-synthetic-draft | 0.931 | 0.295 | 0.850 | 0.149 |
| `clawdlinux-baseline-additive` 1 | 0.342 | 0.008 | 0.988 | 0.376 |

Trainer holdout (76 rows from the same scenarios, optimistic): recall 0.939,
precision 0.868, extra review 0.259, ECE 0.184.

## Shadow-first procedure

1. Mount the artifact. Leave workloads on `shadow`.
2. Collect human decisions. Every escalation still comes from invariants,
   packs, or the threshold.
3. Export and verify the dataset. Run `agentctl decision eval` against the
   human rows with `--baseline`.
4. Retrain on human rows only. Check the new artifact `data_source` is
   `human`.
5. Only then consider `escalate`, per workload, after the checklist below.

## Gate before `escalate` in production

All must hold on a verified export with no synthetic rows:

- [ ] At least 200 human labels, and at least 50 reject or edit labels.
- [ ] Artifact `trained_on.data_source` is `human`.
- [ ] Recall at the chosen threshold beats the baseline and meets the target
      the customer security team signs off.
- [ ] Extra review rate is under the review-load ceiling the reviewing team
      agrees to carry.
- [ ] ECE under 0.05, and no bin with 20+ rows off by more than 0.10.
- [ ] Threshold owner and change process recorded (open item in the ADR).
- [ ] Shadow scores checked against the same period's human decisions.

## Honest status

- The shipped artifact is trained on 30 DRAFT scenarios written by an
  engineer. Founder-written scenarios and real labels must replace them.
- It is not production-validated. Do not enable `escalate` on it.
- Every synthetic row has `escalated_by: rules`, so the `esc_*` features get
  weight 0. Live, the model mostly scores allowed actions, a population no
  human label covers yet.
- Capture mode `none` drops the action name, so `action_destructive` is 0 for
  those examples.
- No calibration step is applied. On the synthetic data, raw scores
  over-predict risk in the low bins and under-predict it in the high bins.
- Runtime-adapter path (`spec.orchestration`) is not scored.
- Phase B bounded autonomy is NOT built. Nothing lets a model auto-allow.
