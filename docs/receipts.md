# Decision receipts

Opt-in. Off by default. Direct action path only.

[Decision tracing](TRACING.md#decision-tracing) joins evaluation spans to signed receipts by sequence and full entry hash.

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
| `human_principal` | `system:clawdlinux-operator`, or the approver identity digest for layer `human` |
| `agent_key_id` | workload UID |

The decision record sits beside the receipt in `records.jsonl`. Fields:
`schema_version`, `workload` (namespace, name, uid), `action`, `layer`
(`invariant`, `rules`, `threshold`, `human`, `model`), `outcome`,
`reasons` (rule ID plus reason), `input_hash`, `payload_sha256`, `declared`, `observed`,
`claimed`, `policy_packs`, `threshold_mode`, `replay_hint`. A `model` block is
added whenever the decision model scored the action. See
[model block](#model-block). Human decisions add an `approval`
block and set `human_principal` to the approver identity digest. See
[approvals](approvals.md).

Records never hold a raw Kubernetes username. Usernames are often emails. The
approver is stored as `sha256:` plus the hex SHA-256 of
`clawdlinux.org/human-identity/v1`, a NUL, and the username. The same digest
goes in `approval.approver`, the `approver` reason, and `human_principal`. To
check a known user, compute the digest and compare. The digest is
pseudonymous, not anonymous: a guessed username can be confirmed.

`input_hash` is the SHA-256 of the canonical JSON of the full decision input:
declared, observed, agent-claimed (labelled `agent_claimed`), and
`payload_sha256`. It covers the raw values without storing them.

`payload_sha256` is the digest of the exact `execute_action` argument map:
`{"action", "params", "confidence"}`. It is `sha256("clawdlinux.decision.payload.v1"
|| 0x00 || canonical JSON)`. Canonical JSON sorts object keys and normalizes
numbers through float64, so a JSON round trip gives the same digest. A human
record carries the digest of the payload the human approved or edited. To
check which payload was authorized, recompute the digest from the payload you
hold and compare. The payload itself is never stored.

Numbers are integers or decimal strings. Never floats. Canonical JSON follows
receiptspec `DECISION.md` (RFC 8785 subset).

## Model block

Present only when `DECISION_MODEL_PATH` loaded an artifact and the workload
mode is `shadow` or `escalate`. Absent otherwise, so records without a model
are byte-identical to before. The record `schema_version` stays 1; the block
has its own `schema_version` (2). See
[decision model](architecture/decision-model.md).

| Field | Meaning |
|---|---|
| `schema_version` | model block version, 2 |
| `mode` | `shadow` or `escalate` |
| `id`, `version`, `artifact_sha256` | the artifact that scored |
| `feature_spec`, `feature_hash` | feature spec version and SHA-256 of the feature vector |
| `option_set` | `["allow", "require_approval"]`, fixed order |
| `option_micro` | per-option probability in micro-units, same order, sums to 1000000 |
| `risk_micro` | P(human rejects or edits), micro-units. The higher of the two below |
| `claimed_risk_micro` | risk with the agent claims as sent |
| `baseline_risk_micro`, `baseline_feature_hash` | risk and feature hash with claimed confidence and health removed. Claims can only raise risk |
| `threshold_micro` | effective threshold, after a stricter workload override |
| `calibration` | calibration note from the artifact |
| `reason_codes` | top 3 positive feature contributions |
| `base_outcome`, `outcome` | outcome before and after the model |
| `error` | scorer error, if any |

In shadow mode `outcome` equals `base_outcome`. In escalate mode the record
`layer` is `model` when the model moved allow to require_approval.

## What is not recorded

- Raw action params and the proposal payload. Only `payload_sha256`.
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
3. On allow, the operator reserves execution: a status write (condition
   `ExecutionReserved`) sent with the resourceVersion the decision read. If
   the workload changed since then, for example a pack was added or declared
   intent was tightened while the append was blocked, the write conflicts.
   The action is not executed. The workload is decided again on the fresh
   object. The receipt from step 2 stays: it records a decision that did not
   run.
4. Only then is the action executed or held.

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
| `RECEIPTS_WRITER_TRUST_FILE` | | Pinned writer public key (trust file). Required when enabled. The operator refuses to start without it |
| `RECEIPTS_INSECURE_NO_PIN` | false | Demo only. Start without a pinned key and accept unverified writer receipts |

With a pinned key, `RemoteWriter.Append` checks every receipt the writer
returns: it binds to the submitted record, its entry hash recomputes, its
signature verifies under a pinned key inside that key's sequence window, and
its seq and prev hash extend the last accepted head by one. The first append
reads `GET /v1/head` to learn the head. A forged, re-signed, or replayed
receipt fails the append, which fails closed under `RECEIPTS_REQUIRED`. The
head read is not signed. A MITM on the first call can shift the start point,
but it still cannot forge a receipt. Appends are serialized per operator.

receipt-writer env:

| Variable | Meaning |
|---|---|
| `RECEIPT_DATA_DIR` | Chain directory. `receipts.jsonl`, `records.jsonl`, `approvals.jsonl`, `writer.lock` |
| `RECEIPT_KEY_FILE` | Hex Ed25519 seed (32 bytes) or private key (64 bytes) |
| `RECEIPT_KEY_GENERATE` | `true` creates the key file if missing. Otherwise the writer refuses to start |
| `RECEIPT_TOKEN_FILE` | Bearer token, at least 32 characters. Read at start |
| `RECEIPT_ADDR` | Listen address, default `:8080` |

Routes: `POST /v1/records`, `GET /v1/head`, `GET /v1/export`,
`POST /v1/approvals`, `GET /v1/approvals` (all bearer auth), `GET /healthz`
(no auth). Record bodies over 64 KiB, approval bodies over 256 KiB, unknown
fields, and trailing data are rejected.

Helm:

```sh
(umask 077 && openssl rand -hex 32 > key.hex)
kubectl -n agentic-system create secret generic receipt-signing-key \
  --from-file=signing-key=key.hex
agentctl receipts trust-root --signing-key-file key.hex > trust.json
kubectl -n agentic-system create configmap receipt-trust-root \
  --from-file=trust.json=trust.json
helm upgrade --install rel charts -n agentic-system \
  --set global.receipts.enabled=true \
  --set global.receipts.required=true \
  --set global.receipts.signingKey.existingSecret=receipt-signing-key \
  --set global.receipts.trustRoot.existingConfigMap=receipt-trust-root
```

Keep `key.hex` offline after this. `trust.json` holds only the public key.
The chart fails to render with receipts enabled and no `trustRoot` unless
`global.receipts.insecureNoPin=true`, which is for demos only.

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

A pass needs a signed export manifest whose head matches the last receipt.
That proves completeness: nobody stripped trailing receipts and records.
An export with no manifest fails with `FAIL: completeness not proven`.
`--allow-prefix` accepts it anyway and prints `WARNING: completeness NOT
proven`. That mode only proves the receipts present are valid. Use it for
partial exports you made yourself, never as evidence of the full history.

The chain alone also verifies with AgentGate:

```sh
agentgate-verify --source jsonl --path ./evidence/receipts.jsonl \
  --trust-root ./pinned-trust.json
```

That checks receipts only, not decision records.

## Gaps

- Runtime-adapter path (`spec.orchestration`) writes no receipts and is not
  scored by the decision model.
- Execution outcome is not receipted. Only the pre-execution decision.
- Human approve, reject, and edit receipts are written on the direct path. See
  [approvals](approvals.md). Argo approval gates are not receipted.
- Key rotation: not supported. The writer refuses a key change.
- Retention and compaction: none. Files grow. Export reads the whole chain.
- Writer transport is plain HTTP inside the cluster, guarded by the
  NetworkPolicy and bearer token. No TLS yet.
- Token rotation needs a writer restart.
- No receipt-writer image is published yet. See `BLOCKERS.md`.
- Writer recovery does not verify signatures or keep an external committed-head checkpoint. Removing the chain tail can fork an issued chain (BLOCKERS.md, F11).
