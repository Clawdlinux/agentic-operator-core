#!/usr/bin/env bash
# Seed the DRAFT synthetic dataset and train the model twice into separate
# directories. Fails unless both runs give byte-identical artifacts.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
out="${1:-$here/out}"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

run() {
	mkdir -p "$1"
	python3 "$here/seed.py" --scenarios "$here/scenarios.json" --out "$1/approvals.jsonl"
	python3 "$here/train.py" --data "$1/approvals.jsonl" --out-dir "$1"
}

run "$tmp/a"
run "$tmp/b"

sum() { python3 -c 'import hashlib,sys;print(hashlib.sha256(open(sys.argv[1],"rb").read()).hexdigest())' "$1"; }
for f in approvals.jsonl model.json metrics.json golden.json; do
	a="$(sum "$tmp/a/$f")"
	b="$(sum "$tmp/b/$f")"
	if [ "$a" != "$b" ]; then
		echo "decision-train: $f differs between runs ($a vs $b)" >&2
		exit 1
	fi
	echo "decision-train: $f identical across 2 runs, sha256 $a"
done

mkdir -p "$out"
cp "$tmp/a/approvals.jsonl" "$tmp/a/model.json" "$tmp/a/metrics.json" "$tmp/a/golden.json" "$out/"
echo "decision-train: wrote $out (synthetic DRAFT data, not production-validated)"
