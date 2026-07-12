# Booth staging

Fallback assets for the Clawdlinux attestation demo. Use these if the live run
hiccups. Do not present from these by default. The live demo is the demo.

## attestation-fallback.jsonl

A real, hash-chained, HMAC-signed audit artifact. 6 entries simulating one
regulated agent run: manifest emit, LLM call, an allowed egress, a denied
egress, a human approval, and a state change to Succeeded. Produced with the
production `pkg/audit` recorder, so it verifies with the shipped `audit-verify`.

This is genuine evidence, not a mock. It verifies clean and rejects tampering.

### Verify it offline (the money shot)

```bash
audit-verify --source jsonl \
  --path _staging/booth/attestation-fallback.jsonl \
  --key booth-demo-2026=bmluZXZpZ2lsLWJvb3RoLWRlbW8tYXR0ZXN0YXRpb24ta2V5LTMyYg==
```

Expect: `PASS — chain is intact and all checkpoints match.`

### Tamper demo (prove it is real evidence)

```bash
sed 's/"actor":"policy-analyst"/"actor":"ghost-actor"/' \
  _staging/booth/attestation-fallback.jsonl > /tmp/tampered.jsonl
audit-verify --source jsonl --path /tmp/tampered.jsonl \
  --key booth-demo-2026=bmluZXZpZ2lsLWJvb3RoLWRlbW8tYXR0ZXN0YXRpb24ta2V5LTMyYg==
```

Expect: `FAIL at seq=4: ... entry_hash mismatch`. One altered field and the
chain breaks. That is what an auditor wants to see.

## Notes

- The key here is a demo key, committed on purpose so the fallback is
  self-contained. Never use this key for a real run.
- The verify works fully offline. No cluster, no network, no external
  dependencies. That is the air-gapped review-room story.
