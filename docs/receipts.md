# Decision receipts

Opt-in. Off by default. Direct action path only.

When receipts are on, the operator writes a signed receipt for every decision
on the legacy direct action path, before it executes the action. The receipt
uses the AgentGate receipt format (`github.com/Clawdlinux/agentgate/pkg/receiptspec`),
so `agentgate-verify` checks the chain without changes. See the
[decision architecture](architecture/decision-architecture.md) build order,
step 3.

## Parts

| Part | What it does |
|---|---|
| `pkg/receipts` | `DecisionRecord`, `Writer` interface, `LocalWriter` (file chain), `RemoteWriter` (HTTP client), export and offline verify |
| `cmd/receipt-writer` | HTTP service. Owns the Ed25519 signing key and the chain. Single writer |
| Controller | Builds the record, calls `Append`, then executes or denies |
| `agentctl receipts` | `export` from the writer, `verify` offline |
| Chart | `global.receipts` block: StatefulSet, PVC, Service, token Secret, NetworkPolicy |

The operator never holds the signing key. It sends records to the writer with
a bearer token.

## What is recorded

Each receipt is a v1 AgentGate receipt:

| Receipt field | Value |
|---|---|
| `service` | `clawdlinux` |
| `action` | `decision:<action name>` (clipped to 128 bytes) |
| `policy_decision` | `allow` for allow. `deny` for deny and require_approval |
| `status_code` | 200 allow, 202 require_approval, 403 deny |
| `params_sha256` | SHA-256 of the canonical decision record |
| `human_principal` | `system:clawdlinux-operator` |
| `agent_key_id` | workload UID |

The decision record sits beside the receipt in `records.jsonl`. Fields:
`schema_version`, `workload` (namespace, name, uid), `action`, `layer`
(`invariant`, `rules`, `threshold`; `human` and `model` reserved), `outcome`,
`reasons` (rule ID plus reason), `input_hash`, `declared`, `observed`,
`claimed`, `policy_packs`, `threshold_mode`, `replay_hint`. A `model` block is
reserved and absent until a model decides.

`input_hash` is the SHA-256 of the canonical JSON of the full decision input:
declared, observed, and agent-claimed (labelled `agent_claimed`). It covers
the raw values without storing them.

Numbers are integers or decimal strings. Never floats. Canonical JSON follows
receiptspec `DECISION.md` (RFC 8785 subset).

## What is not recorded

- Raw action params and the proposal payload.
- Matched content: no email, phone, card, or credential value. Only class
  names like `email`.
- Declared purpose, agent-claimed intent, caller identity. Hashed only.
- Destination path and query. Only the host.
- Execution outcome. The receipt records the decision, not what the MCP
  server did after.

Reason strings come from invariants, packs, and the threshold evaluator. They
name classes and hosts, never matched values. A pack load error message is
recorded as is.

## Write-ahead and fail closed

Order on the direct path:

1. Invariants, packs, threshold run. `decision.Decide` picks the outcome.
2. The operator appends the receipt. `Append` returns only after the record
   and receipt are fsynced on the writer.
3. Only then is the action executed or held.

`RECEIPTS_REQUIRED=true`:

- Writer not configured or not ready (`/healthz` not 200): INV-05 denies. The
  operator still tries to append the deny record.
- `Append` fails or times out, for example a bad token: INV-05 is added and
  the decision is re-run. The outcome becomes deny. The action is not executed.
- In strict mode the workload goes to `PolicyDenied`. Otherwise
  `PendingApproval`, same as any invariant.
- The INV-05 deny itself has no receipt, because the writer just failed.

`RECEIPTS_ENABLED=true`, not required: an append failure is logged and the
action proceeds. Use this only to try receipts out. It gives no guarantee that
every action has a receipt.

Receipts disabled (default): nothing changes.

## Configuration

Operator env:

| Variable | Default | Meaning |
|---|---|---|
| `RECEIPTS_ENABLED` | false | Write receipts |
| `RECEIPTS_REQUIRED` | false | Fail closed. Implies enabled |
| `RECEIPTS_WRITER_URL` | | Writer base URL |
| `RECEIPTS_WRITER_TOKEN_FILE` | | Bearer token file, read per request |
| `RECEIPTS_WRITER_TIMEOUT` | 5s | Per request timeout |

receipt-writer env:

| Variable | Meaning |
|---|---|
| `RECEIPT_DATA_DIR` | Chain directory. `receipts.jsonl`, `records.jsonl`, `writer.lock` |
| `RECEIPT_KEY_FILE` | Hex Ed25519 seed (32 bytes) or private key (64 bytes) |
| `RECEIPT_KEY_GENERATE` | `true` creates the key file if missing. Otherwise the writer refuses to start |
| `RECEIPT_TOKEN_FILE` | Bearer token, at least 32 characters. Read at start |
| `RECEIPT_ADDR` | Listen address, default `:8080` |

Routes: `POST /v1/records`, `GET /v1/head`, `GET /v1/export` (all bearer
auth), `GET /healthz` (no auth). Bodies over 64 KiB, unknown fields, and
trailing data are rejected.

Helm:

```sh
kubectl -n agentic-system create secret generic receipt-signing-key \
  --from-literal=signing-key=$(openssl rand -hex 32)
helm upgrade --install rel charts -n agentic-system \
  --set global.receipts.enabled=true \
  --set global.receipts.required=true \
  --set global.receipts.signingKey.existingSecret=receipt-signing-key
```

The block lives under `global` because the operator Deployment is a subchart.
The chart never generates the signing key. It creates the token Secret once
unless `token.existingSecret` is set. A NetworkPolicy lets only operator pods
reach the writer and blocks writer egress.

## Key custody

- The key lives in one Secret, mounted read-only into one pod.
- Keep an offline copy. Losing it means new receipts start a chain that old
  trust files do not cover.
- The writer refuses to resume a chain whose head was signed by another key.
  Rotation is not supported yet.
- Publish the trust root (`trust.json` from an export, or the `kid` and public
  key) through a channel you trust, once. Verify against that pinned copy.

## Verify offline

```sh
agentctl receipts export --writer http://localhost:8080 \
  --token-file ./token --out ./evidence
agentctl receipts verify ./evidence --trust-root ./pinned-trust.json
```

`verify` runs `receiptspec.VerifyBundle`, the check `agentgate-verify` runs,
then `receiptspec.VerifyRecordBinding` for every record. It prints PASS or
FAIL lines and exits nonzero on failure. Without `--trust-root` it falls back
to the export's own `trust.json`, which proves consistency, not origin.

The chain alone also verifies with AgentGate:

```sh
agentgate-verify --source jsonl --path ./evidence/receipts.jsonl \
  --trust-root ./pinned-trust.json
```

That checks receipts only, not decision records.

## Gaps

- Runtime-adapter path (`spec.orchestration`) writes no receipts.
- Execution outcome is not receipted. Only the pre-execution decision.
- Human approve, reject, and edit receipts are not written yet. The outcomes
  exist in the record schema.
- Key rotation: not supported. The writer refuses a key change.
- Retention and compaction: none. Files grow. Export reads the whole chain.
- Writer transport is plain HTTP inside the cluster, guarded by the
  NetworkPolicy and bearer token. No TLS yet.
- Token rotation needs a writer restart.
- No receipt-writer image is published yet. See `BLOCKERS.md`.
