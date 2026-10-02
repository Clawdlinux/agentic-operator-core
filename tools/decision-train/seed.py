#!/usr/bin/env python3
"""Expand scenarios.json into a labelled synthetic approvals.jsonl.

Every row has the approvals.jsonl Example schema plus "source": "synthetic".
Synthetic rows bind to no receipt, so `agentctl dataset verify` and
`agentctl decision eval` treat them as unverified. Stdlib only.

Usage: seed.py --scenarios scenarios.json --out out/approvals.jsonl
"""

import argparse
import hashlib
import json
import random

LABELS = ["approve", "reject", "edit"]


def sha(s):
    return hashlib.sha256(s.encode()).hexdigest()


def pick_int(rng, lo_hi):
    lo, hi = lo_hi
    return rng.randint(lo, hi)


def row(sc, i, rng, noise_micro):
    dest = rng.choice(sc["destinations"])
    conf = ""
    if sc["confidence"] is not None:
        lo, hi = sc["confidence"]
        conf = repr(round(rng.uniform(lo, hi), 2))
    health = ""
    if sc["cluster_health"] is not None:
        health = str(pick_int(rng, sc["cluster_health"]))
    actions = pick_int(rng, sc["prior_action_count"])
    denied = min(pick_int(rng, sc["prior_denied_count"]), actions) if actions > 0 else 0
    label = sc["label"]
    if rng.randrange(1_000_000) < noise_micro:
        label = rng.choice([x for x in LABELS if x != label])
    ex_id = "syn-%s-%03d" % (sc["id"], i)
    content = {
        "action_sha256": sha(sc["action"]),
        "data_classes": sorted(sc["data_classes"]),
        "name": sc["action"],
    }
    ex = {
        "schema_version": 1,
        "example_id": ex_id,
        "receipt_seq": 0,
        "receipt_entry_hash": "",
        "workload": {"namespace": "synthetic", "name": sc["id"].lower(), "uid": ""},
        "features": {
            "original_seq": 0,
            "original_input_hash": "",
            "decision_type": sc["decision_type"],
            "allowed_data_classes": sorted(sc["allowed_data_classes"]),
            "allowed_destinations": sorted(sc["allowed_destinations"]),
            "observed_destination": dest,
            "observed_data_classes": sorted(sc["data_classes"]),
            "tool": sc["action"],
            "prior_action_count": actions,
            "prior_denied_count": denied,
            "claimed_confidence": conf,
            "claimed_cluster_health": health,
            "policy_packs": sorted(sc["policy_packs"]),
            "threshold_mode": "strict",
            "escalated_by": "rules",
            "reason_codes": ["synthetic"],
        },
        "label": label,
        "reason_sha256": sha(sc["rationale"]),
        "approver_sha256": sha("synthetic"),
        "timestamp": "1970-01-01T00:00:00Z",
        "capture_mode": "redacted",
        "content": content,
        "source": "synthetic",
    }
    if label == "edit":
        ex["edit_diff"] = {
            "fields_changed": ["params"],
            "edited": {"action_sha256": sha(sc["action"] + ":edited"), "data_classes": [], "name": sc["action"]},
        }
    return ex


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--scenarios", required=True)
    ap.add_argument("--out", required=True)
    args = ap.parse_args()
    with open(args.scenarios) as f:
        doc = json.load(f)
    lines = []
    for idx, sc in enumerate(doc["scenarios"]):
        rng = random.Random(doc["seed"] * 1000 + idx)
        for i in range(doc["rows_per_scenario"]):
            lines.append(json.dumps(row(sc, i, rng, doc["label_noise_micro"]), sort_keys=True, separators=(",", ":")))
    with open(args.out, "w") as f:
        f.write("\n".join(lines) + "\n")
    print("seed: wrote %d synthetic rows from %d DRAFT scenarios to %s" % (len(lines), len(doc["scenarios"]), args.out))


if __name__ == "__main__":
    main()
