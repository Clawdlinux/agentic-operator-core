#!/usr/bin/env python3
"""Train the escalate-only decision model on approvals.jsonl data.

Logistic regression, full-batch gradient descent, fixed seed and fixed
iteration count, so the same data gives the same artifact bytes. Label 1
means a human rejected or edited the action. Weights are rounded to 6
decimals and written as strings. Metrics use the same integer inference as
the Go operator (pkg/decision/learned), so the numbers match exactly.
Stdlib only.

Usage: train.py --data out/approvals.jsonl --out-dir out [--threshold-micro N]
"""

import argparse
import hashlib
import json
import math
import os

from features import FEATURE_NAMES, FEATURE_SPEC_VERSION, features, matrix_sha256

MICRO = 1_000_000
EXP_SCALE = 1_000_000_000
E_INV = 367_879_441
MAX_LOGIT = 30 * MICRO
HOLDOUT_MOD = 5  # one in five examples, chosen by example_id hash


def exp_neg(x):
    """e^(-x) at EXP_SCALE for x >= 0 in micro-units. Same steps as Go."""
    n = x // MICRO
    f = (x % MICRO) * 1000
    s, term = EXP_SCALE, EXP_SCALE
    for k in range(1, 26):
        term = term * f // (EXP_SCALE * k)
        if term == 0:
            break
        s = s - term if k % 2 == 1 else s + term
    for _ in range(n):
        s = s * E_INV // EXP_SCALE
    return s


def sigmoid_micro(z):
    z = max(-MAX_LOGIT, min(MAX_LOGIT, z))
    neg = z < 0
    e = exp_neg(-z if neg else z)
    den = EXP_SCALE + e
    if neg:
        return (e * MICRO + den // 2) // den
    return (EXP_SCALE * MICRO + den // 2) // den


def fmt6(w):
    s = "%.6f" % w
    return "0.000000" if s == "-0.000000" else s


def parse_micro(s):
    neg = s.startswith("-")
    ip, _, fr = s.lstrip("-").partition(".")
    v = int(ip) * MICRO + int((fr + "000000")[:6])
    return -v if neg else v


def logit_micro(wm, bm, x):
    return bm + sum(w * v for w, v in zip(wm, x))


def fit(X, y, active, iters, lr, l2):
    n, d = len(X), len(FEATURE_NAMES)
    w, b = [0.0] * d, 0.0
    for _ in range(iters):
        gw, gb = [0.0] * d, 0.0
        for xi, yi in zip(X, y):
            z = b + sum(w[j] * xi[j] for j in active)
            z = max(-30.0, min(30.0, z))
            err = 1.0 / (1.0 + math.exp(-z)) - yi
            gb += err
            for j in active:
                gw[j] += err * xi[j]
        for j in active:
            w[j] -= lr * (gw[j] / n + l2 * w[j])
        b -= lr * gb / n
    return w, b


def evaluate(risks, y, threshold):
    tp = fp = tn = fn = 0
    for r, t in zip(risks, y):
        esc = r >= threshold
        if esc and t:
            tp += 1
        elif esc:
            fp += 1
        elif t:
            fn += 1
        else:
            tn += 1
    bins = []
    n = len(risks)
    ece_num = 0
    for k in range(10):
        lo, hi = k * 100_000, (k + 1) * 100_000
        idx = [i for i, r in enumerate(risks) if min(r // 100_000, 9) == k]
        c = len(idx)
        mean = sum(risks[i] for i in idx) // c if c else 0
        obs = sum(y[i] for i in idx) * MICRO // c if c else 0
        ece_num += c * abs(mean - obs)
        bins.append({"lower_micro": lo, "upper_micro": hi, "count": c, "mean_predicted_micro": mean, "observed_micro": obs})
    return {
        "examples": n,
        "positives": tp + fn,
        "negatives": fp + tn,
        "true_positive": tp,
        "false_positive": fp,
        "true_negative": tn,
        "false_negative": fn,
        "recall_micro": tp * MICRO // (tp + fn) if tp + fn else 0,
        "precision_micro": tp * MICRO // (tp + fp) if tp + fp else 0,
        "extra_review_rate_micro": fp * MICRO // (fp + tn) if fp + tn else 0,
        "ece_micro": ece_num // n if n else 0,
        "bins": bins,
    }


def canonical_sha(obj):
    o = dict(obj)
    o.pop("artifact_sha256", None)
    raw = json.dumps(o, sort_keys=True, separators=(",", ":"))
    return hashlib.sha256(raw.encode()).hexdigest()


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--data", required=True)
    ap.add_argument("--out-dir", required=True)
    ap.add_argument("--threshold-micro", type=int, default=500_000)
    ap.add_argument("--iterations", type=int, default=2000)
    ap.add_argument("--learning-rate", default="0.3")
    ap.add_argument("--l2", default="0.01")
    ap.add_argument("--model-version", default="0.1.0-synthetic-draft")
    args = ap.parse_args()

    with open(args.data, "rb") as f:
        raw = f.read()
    rows = [json.loads(l) for l in raw.decode().splitlines() if l.strip()]
    X = [features(r) for r in rows]
    y = [1 if r["label"] in ("reject", "edit") else 0 for r in rows]
    hold = [int(hashlib.sha256(r["example_id"].encode()).hexdigest()[:8], 16) % HOLDOUT_MOD == 0 for r in rows]
    Xtr = [x for x, h in zip(X, hold) if not h]
    ytr = [t for t, h in zip(y, hold) if not h]
    # A feature constant in the training split keeps weight 0. Otherwise it
    # would soak up the bias and skew live scores where it differs.
    active = [j for j in range(len(FEATURE_NAMES)) if len({x[j] for x in Xtr}) > 1]

    w, b = fit(Xtr, ytr, active, args.iterations, float(args.learning_rate), float(args.l2))
    weights = [fmt6(v) for v in w]
    bias = fmt6(b)
    wm = [parse_micro(s) for s in weights]
    bm = parse_micro(bias)
    risks = [sigmoid_micro(logit_micro(wm, bm, x)) for x in X]
    rh = [r for r, h in zip(risks, hold) if h]
    yh = [t for t, h in zip(y, hold) if h]
    rt = [r for r, h in zip(risks, hold) if not h]
    holdout = evaluate(rh, yh, args.threshold_micro)
    train = evaluate(rt, ytr, args.threshold_micro)

    sources = [r.get("source", "") for r in rows]
    synth = sum(1 for s in sources if s == "synthetic")
    human = sum(1 for s in sources if s == "")
    data_source = "synthetic" if human == 0 else ("human" if synth == 0 else "mixed")
    art = {
        "schema_version": 1,
        "model_id": "clawdlinux-escalate-logreg",
        "version": args.model_version,
        "model_type": "logistic_regression",
        "feature_spec_version": FEATURE_SPEC_VERSION,
        "features": FEATURE_NAMES,
        "weights": weights,
        "bias": bias,
        "threshold_micro": args.threshold_micro,
        "calibration": {"method": "none", "split": "holdout", "bins": holdout["bins"], "ece_micro": holdout["ece_micro"]},
        "trained_on": {
            "data_source": data_source,
            "dataset_sha256": hashlib.sha256(raw).hexdigest(),
            "examples_total": len(rows),
            "examples_train": len(Xtr),
            "examples_holdout": len(rh),
            "positives": sum(ytr),
            "negatives": len(ytr) - sum(ytr),
            "synthetic_rows": synth,
            "human_rows": human,
        },
        "training": {
            "method": "full-batch gradient descent",
            "iterations": str(args.iterations),
            "learning_rate": args.learning_rate,
            "l2": args.l2,
            "split": "holdout when sha256(example_id)[:8] mod %d == 0" % HOLDOUT_MOD,
            "label": "1 when a human rejected or edited",
        },
    }
    art["artifact_sha256"] = canonical_sha(art)
    metrics = {
        "model_id": art["model_id"],
        "version": art["version"],
        "artifact_sha256": art["artifact_sha256"],
        "data_source": data_source,
        "threshold_micro": args.threshold_micro,
        "feature_matrix_sha256": matrix_sha256(X),
        "active_features": [FEATURE_NAMES[j] for j in active],
        "train": train,
        "holdout": holdout,
    }
    golden = []
    for r, x in list(zip(rows, X))[:: max(1, len(rows) // 30)]:
        golden.append({"example_id": r["example_id"], "features": x, "logit_micro": logit_micro(wm, bm, x), "risk_micro": sigmoid_micro(logit_micro(wm, bm, x))})
    for z in (-40_000_000, -30_000_000, -5_500_000, -1, 0, 1, 250_000, 999_999, 1_000_000, 7_123_456, 30_000_000, 40_000_000):
        golden.append({"example_id": "sigmoid", "features": [], "logit_micro": z, "risk_micro": sigmoid_micro(z)})

    def dump(name, obj):
        with open(os.path.join(args.out_dir, name), "w") as f:
            f.write(json.dumps(obj, indent=2, sort_keys=True) + "\n")

    dump("model.json", art)
    dump("metrics.json", metrics)
    dump("golden.json", golden)
    h = holdout
    print("train: %s data, %d rows (%d train, %d holdout), artifact sha256 %s" % (data_source, len(rows), len(Xtr), len(rh), art["artifact_sha256"]))
    print("holdout at threshold %d micro: recall %d, precision %d, extra review %d, ECE %d (micro)" % (
        args.threshold_micro, h["recall_micro"], h["precision_micro"], h["extra_review_rate_micro"], h["ece_micro"]))


if __name__ == "__main__":
    main()
