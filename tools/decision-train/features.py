"""Feature extraction for the decision model, spec v1.

Mirrors pkg/decision/features.go exactly. If one changes, the other must
change in the same commit and FEATURE_SPEC_VERSION must move. A Go test
recomputes feature_matrix_sha256 from the shipped dataset to catch drift.
Stdlib only.
"""

import hashlib

FEATURE_SPEC_VERSION = "v1"

FEATURE_NAMES = [
    "dc_personal_count",
    "dc_credential",
    "dc_undeclared_count",
    "personal_to_undeclared",
    "dest_present",
    "dest_declared",
    "declared_present",
    "decision_automated",
    "action_destructive",
    "prior_actions_bucket",
    "prior_denied",
    "conf_absent",
    "conf_low",
    "conf_mid",
    "health_absent",
    "health_low",
    "health_mid",
    "pack_any",
    "pack_dpdp_in",
    "pack_gdpr_eu",
    "esc_invariant",
    "esc_rules",
    "esc_threshold",
    "esc_model",
]

PERSONAL = {"email", "phone", "aadhaar", "pan", "iban", "card"}
DESTRUCTIVE = {"cleanup", "clear", "delete", "destroy", "drop", "purge", "remove", "reset", "truncate", "wipe"}


def _host(dest):
    # Dataset rows already hold a bare host (receipts store the host only).
    return dest.strip().strip("[]").lower()


def _host_allowed(host, patterns):
    for p in patterns:
        p = p.strip().lower()
        if not p:
            continue
        if p.startswith("*."):
            suffix = p[1:]
            if host.endswith(suffix) and len(host) > len(suffix):
                return True
            continue
        if host == p:
            return True
    return False


def _tokens(name):
    out, cur = [], ""
    for ch in name.lower():
        if ch in "_-./ ":
            if cur:
                out.append(cur)
            cur = ""
        else:
            cur += ch
    if cur:
        out.append(cur)
    return out


def _bucket(n):
    if n <= 0:
        return 0
    if n < 10:
        return 1
    if n < 100:
        return 2
    return 3


def _cap(n, m):
    return max(0, min(n, m))


def features(ex):
    """Feature vector (list of ints) for one approvals.jsonl example."""
    f = ex["features"]
    dt = f.get("decision_type", "")
    adc = f.get("allowed_data_classes") or []
    ads = f.get("allowed_destinations") or []
    odc = f.get("observed_data_classes") or []
    declared = dt != "" or len(adc) > 0 or len(ads) > 0

    personal = set()
    credential = False
    for c in odc:
        c = c.strip().lower()
        if c in PERSONAL:
            personal.add(c)
        if c == "credential":
            credential = True
    undeclared = 0
    if declared:
        allowed = {a.strip().lower() for a in adc}
        seen = set()
        for c in odc:
            c = c.strip().lower()
            if c not in allowed and c not in seen:
                seen.add(c)
                undeclared += 1
    host = _host(f.get("observed_destination", ""))
    dest_declared = host != "" and _host_allowed(host, ads)

    v = {
        "dc_personal_count": _cap(len(personal), 6),
        "dc_credential": int(credential),
        "dc_undeclared_count": _cap(undeclared, 6),
        "personal_to_undeclared": int(len(personal) > 0 and host != "" and not dest_declared),
        "dest_present": int(host != ""),
        "dest_declared": int(dest_declared),
        "declared_present": int(declared),
        "decision_automated": int(dt.strip().lower() == "automated"),
        "action_destructive": int(any(t in DESTRUCTIVE for t in _tokens(ex.get("content", {}).get("name", "")))),
        "prior_actions_bucket": _bucket(f.get("prior_action_count", 0)),
        "prior_denied": _cap(f.get("prior_denied_count", 0), 10),
    }
    conf = f.get("claimed_confidence", "")
    if conf == "":
        v["conf_absent"] = 1
    elif float(conf) < 0.5:
        v["conf_low"] = 1
    elif float(conf) < 0.8:
        v["conf_mid"] = 1
    health = f.get("claimed_cluster_health", "")
    if health == "":
        v["health_absent"] = 1
    elif float(health) < 50:
        v["health_low"] = 1
    elif float(health) < 80:
        v["health_mid"] = 1
    for p in f.get("policy_packs") or []:
        v["pack_any"] = 1
        name = p.lower().split("@", 1)[0]
        if name == "dpdp-in":
            v["pack_dpdp_in"] = 1
        elif name == "gdpr-eu":
            v["pack_gdpr_eu"] = 1
    esc = f.get("escalated_by", "")
    if esc == "invariant":
        v["esc_invariant"] = 1
    elif esc in ("rules", "pack"):
        v["esc_rules"] = 1
    elif esc == "threshold":
        v["esc_threshold"] = 1
    elif esc == "model":
        v["esc_model"] = 1
    return [v.get(n, 0) for n in FEATURE_NAMES]


def matrix_sha256(rows):
    """SHA-256 hex over one "v1,v2,...\\n" line per row, in order."""
    h = hashlib.sha256()
    for r in rows:
        h.update((",".join(str(x) for x in r) + "\n").encode())
    return h.hexdigest()
