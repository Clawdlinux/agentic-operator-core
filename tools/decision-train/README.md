# decision-train

Trains the escalate-only decision model. Python 3, standard library only.
See [docs/architecture/decision-model.md](../../docs/architecture/decision-model.md).

## Status: DRAFT, synthetic, not production-validated

`scenarios.json` holds 30 DRAFT scenarios written by an engineer, not the
founder. They exist so shadow mode has something to run. Replace them with
founder-written scenarios, then with real human labels from approvals.jsonl.
Every number this tool prints on the shipped data measures how well a model
fits rules we wrote ourselves. It says nothing about real reviewers.

The scenarios file is JSON, not YAML. The trainer is stdlib only and Python
has no YAML parser in the standard library.

## Files

| File | What it does |
| --- | --- |
| `scenarios.json` | 30 DRAFT scenarios, each with a label (approve, reject, edit) and a rationale |
| `seed.py` | Expands each scenario into 12 seeded rows in the approvals.jsonl Example schema, `source: "synthetic"`, 5% label noise |
| `features.py` | Feature spec v1. Mirrors `pkg/decision/features.go` |
| `train.py` | Logistic regression by full-batch gradient descent, fixed iterations, 1 in 5 held out by example_id hash |
| `run.sh` | Seeds and trains twice, fails unless every output is byte-identical |
| `out/` | Shipped dataset, `model.json` artifact, `metrics.json`, `golden.json` |

## Run

```sh
make decision-train
# or train on a real export
python3 tools/decision-train/train.py --data export/approvals.jsonl --out-dir /tmp/model --model-version 0.2.0
```

Label 1 means a human rejected or edited. Approve is 0.

## Determinism

- Fixed seed, fixed iteration count, fixed holdout rule. No clock, no
  randomness in training.
- Weights are rounded to 6 decimals and stored as strings. Inference in Go
  and in `train.py` uses only integers (fixed-point exp and sigmoid), so the
  metrics here match the operator bit for bit.
- `artifact_sha256` is SHA-256 over the artifact as compact JSON with sorted
  keys and that field removed. The operator recomputes it and refuses a
  mismatch.
- `golden.json` holds Python-computed features, logits, and risks. A Go test
  checks the operator gets the same values. `metrics.json`
  `feature_matrix_sha256` catches drift between `features.py` and
  `features.go`.

## Honest limits

- Training data is synthetic. Holdout rows come from the same 30 scenarios
  as training rows, so holdout scores are optimistic.
- Every synthetic row has `escalated_by: rules`. That feature is constant,
  so it gets weight 0. Live, the model mostly scores actions that were
  allowed, a population no human label covers yet.
- Capture mode `none` drops the action name, so those rows lose
  `action_destructive`.
- No calibration step is applied. The table in the artifact only reports
  how far off the raw scores are.
- Logistic regression has no feature interactions beyond the ones encoded by
  hand (`personal_to_undeclared`).
