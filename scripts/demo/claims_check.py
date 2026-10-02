#!/usr/bin/env python3
"""Read-only checks for scripts/demo-claims.sh. Stdlib only. Demo only."""
import hashlib
import json
import os
import sys


def jsonl(path):
    out = []
    with open(path, encoding="utf-8") as f:
        for line in f:
            line = line.strip()
            if not line.startswith("{"):
                continue
            try:
                out.append(json.loads(line))
            except ValueError:
                continue
    return out


def records(export_dir, workload):
    """Decision records for a workload, joined to their receipt."""
    receipts = {r["seq"]: r for r in jsonl(os.path.join(export_dir, "receipts.jsonl"))
                if r.get("type") == "receipt"}
    rows = []
    for line in jsonl(os.path.join(export_dir, "records.jsonl")):
        rec = line["record"]
        if rec["workload"]["name"] != workload:
            continue
        rows.append((line["seq"], rec, receipts.get(line["seq"], {})))
    return sorted(rows, key=lambda r: r[0])


def cmd_records(export_dir, workload):
    for seq, rec, rc in records(export_dir, workload):
        ids = ",".join(r["rule_id"] for r in rec.get("reasons") or [])
        print(f"seq={seq} layer={rec['layer']} outcome={rec['outcome']} "
              f"policy_decision={rc.get('policy_decision')} principal={rc.get('human_principal')} "
              f"ts_ns={rc.get('timestamp_unix_ns')} rules=[{ids}] "
              f"claimed_confidence={rec.get('claimed', {}).get('confidence')} "
              f"claimed_health={rec.get('claimed', {}).get('cluster_health')}")


def tool_calls(log, scenario, tool):
    return [l for l in jsonl(log) if l.get("scenario") == scenario and l.get("tool") == tool]


def cmd_calls(log, scenario, tool):
    print(len(tool_calls(log, scenario, tool)))


def cmd_write_ahead(export_dir, workload, log, scenario):
    """Exactly one receipt for the workload precedes the first execute."""
    execs = tool_calls(log, scenario, "execute_action")
    if not execs:
        print("FAIL no execute_action call")
        return 1
    first = min(e["unix_ns"] for e in execs)
    before = [(s, rec, rc) for s, rec, rc in records(export_dir, workload)
              if int(rc.get("timestamp_unix_ns", 0)) < first]
    for s, rec, rc in before:
        print(f"receipt seq={s} outcome={rec['outcome']} ts_ns={rc['timestamp_unix_ns']}")
    print(f"first execute_action unix_ns={first}")
    if len(before) != 1 or before[0][1]["outcome"] != "allow":
        print(f"FAIL want exactly 1 allow receipt before execute, got {len(before)}")
        return 1
    print(f"receipt precedes execute by {(first - int(before[0][2]['timestamp_unix_ns'])) / 1e6:.1f} ms")
    return 0


def cmd_model(export_dir, workload, model_path):
    with open(model_path, encoding="utf-8") as f:
        artifact = json.load(f)
    rows = [r for r in records(export_dir, workload) if r[1].get("model")]
    if not rows:
        print("FAIL no model block")
        return 1
    seq, rec, _ = rows[0]
    m = rec["model"]
    print(json.dumps({"seq": seq, "record_outcome": rec["outcome"], "model": {
        k: m.get(k) for k in ("mode", "id", "version", "artifact_sha256", "risk_micro",
                              "threshold_micro", "option_set", "option_micro",
                              "base_outcome", "outcome")}}, indent=2))
    ok = (m["mode"] == "shadow" and m["artifact_sha256"] == artifact["artifact_sha256"]
          and isinstance(m["risk_micro"], int) and m["outcome"] == m["base_outcome"]
          and rec["outcome"] == "allow" and all(isinstance(x, int) for x in m["option_micro"]))
    print("model block checks:", "ok" if ok else "FAIL")
    return 0 if ok else 1


def cmd_approvals(path, workload):
    rows = [r for r in jsonl(path) if r.get("workload", {}).get("name") == workload]
    for r in rows:
        print(f"example_id={r['example_id']} label={r['label']} receipt_seq={r['receipt_seq']} "
              f"capture_mode={r.get('capture_mode')}")
    print(len(rows))


def cmd_flip(path, needle):
    """Flip one bit of the byte right after the first needle."""
    with open(path, "rb") as f:
        data = bytearray(f.read())
    i = data.find(needle.encode())
    if i < 0:
        print(f"needle {needle!r} not found")
        return 1
    i += len(needle)
    old = data[i]
    data[i] ^= 0x01
    with open(path, "wb") as f:
        f.write(data)
    print(f"flipped byte {i}: {chr(old)!r} -> {chr(data[i])!r}")
    return 0


def cmd_digest(username):
    """IdentityDigest from pkg/receipts/record.go."""
    data = ("clawdlinux.org/human-identity/v1\x00" + username).encode()
    print("sha256:" + hashlib.sha256(data).hexdigest())


def cmd_strip_manifest(path):
    """Drop the signed export manifest line from a receipts.jsonl."""
    with open(path, encoding="utf-8") as f:
        lines = f.readlines()
    kept = []
    for line in lines:
        try:
            if json.loads(line).get("type") == "manifest":
                continue
        except ValueError:
            pass
        kept.append(line)
    with open(path, "w", encoding="utf-8") as f:
        f.writelines(kept)
    removed = len(lines) - len(kept)
    print(f"removed {removed} manifest line(s)")
    return 0 if removed else 1


def main(argv):
    cmds = {
        "records": (cmd_records, 2),
        "calls": (cmd_calls, 3),
        "write-ahead": (cmd_write_ahead, 4),
        "model": (cmd_model, 3),
        "approvals": (cmd_approvals, 2),
        "flip": (cmd_flip, 2),
        "digest": (cmd_digest, 1),
        "strip-manifest": (cmd_strip_manifest, 1),
    }
    if len(argv) < 2 or argv[1] not in cmds or len(argv) - 2 != cmds[argv[1]][1]:
        print("usage: claims_check.py " + "|".join(cmds) + " args...", file=sys.stderr)
        return 2
    rc = cmds[argv[1]][0](*argv[2:])
    return rc or 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
