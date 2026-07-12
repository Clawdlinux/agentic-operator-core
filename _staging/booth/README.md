# Booth evidence boundaries

The booth flow has 6 evidence items. State the boundary before each step.

- **REAL:** The AgentWorkload makes one OpenAI-routed call through in-cluster LiteLLM.
- **REAL:** Its status reports genuine nonzero tokens. Its cost metric and annotation are nonzero.
- **REAL, SEPARATE CHECK:** A LiteLLM request checks Anthropic reachability.
- **SIMULATION / CONFIGURATION PROOF:** A server-side dry-run shows gVisor mutation.
- **NETWORKPOLICY OBJECT PRESENCE ONLY:** The script lists the NetworkPolicy object.
- **PRIOR-RUN ARTIFACT:** The checked-in HMAC-signed audit fixture verifies offline.

The current AgentWorkload does not generate the audit fixture. Audit recording
is not wired into reconciliation yet.
The `--present` path does not execute OPA allow or deny evaluation.
OPA remains in the legacy default flow and target product contract.

## Credential loading

`scripts/demo-booth.sh --prepare` reads provider keys from exported variables first.
Missing keys can load from the repo-root `.env` file.

Set `DEMO_ENV_FILE=/path/to/file` to use another file.
The parser accepts only `OPENAI_API_KEY` and `ANTHROPIC_API_KEY` definitions.
It supports optional `export` prefixes and quoted values.
It does not source the file or evaluate its contents.

Preparation prints each variable name as `available` and reports its source.
It never prints credential values.

## Prior-run audit fixture

`attestation-fallback.jsonl` is a real hash-chained, HMAC-signed artifact.
It was produced earlier with `pkg/audit`. It contains 6 audit entries.

### Verify it offline

```bash
audit-verify --source jsonl \
  --path _staging/booth/attestation-fallback.jsonl \
  --key booth-demo-2026=bmluZXZpZ2lsLWJvb3RoLWRlbW8tYXR0ZXN0YXRpb24ta2V5LTMyYg==
```

Expect a pass showing the chain is intact.

### Optional tamper proof

```bash
sed 's/"actor":"policy-analyst"/"actor":"ghost-actor"/' \
  _staging/booth/attestation-fallback.jsonl > /tmp/tampered.jsonl
audit-verify --source jsonl --path /tmp/tampered.jsonl \
  --key booth-demo-2026=bmluZXZpZ2lsLWJvb3RoLWRlbW8tYXR0ZXN0YXRpb24ta2V5LTMyYg==
```

Expect a failure at the changed entry. The script removes its temporary file.

## Notes

- The committed key is only for this fixture.
- Never use it for a real workload.
- Verification works offline without the cluster.
