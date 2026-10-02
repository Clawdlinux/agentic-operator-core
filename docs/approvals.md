# Human approvals and the approval dataset

Direct action path only. Approvals need receipts on for the signed dataset.

[Decision tracing](TRACING.md#decision-tracing) covers human approve, reject, and edit without exposing approver names or reasons.

When the operator holds an action in `PendingApproval`, a human decides it.
Each decision is signed as a receipt and stored as one labelled example. This
is layer 3 of the [decision architecture](architecture/decision-architecture.md).
The dataset feeds the decision model later (build order step 5).

## Protocol

A held action is stored in `status.pendingApproval` with an `id`. The
workload stops re-proposing while it waits. A human records a decision with
annotations:

| Annotation | Set by | Value |
|---|---|---|
| `clawdlinux.org/approval-decision` | client | `approve`, `reject`, or `edit` |
| `clawdlinux.org/approval-reason` | client | Optional free text, at most 1024 bytes |
| `clawdlinux.org/approval-edit` | client | For `edit`: `{"name", "description", "params"}`, at most 16 KiB |
| `clawdlinux.org/approval-by` | webhook | JSON `{"username", "groups_sha256"}` from the admission request |
| `clawdlinux.org/approval-for` | webhook | The pending action `id` the decision is for |
| `clawdlinux.org/approval-mac` | webhook | Hex HMAC-SHA256 over the stamp, see below |

With agentctl:

```sh
agentctl approve my-workload --reason "checked the target"
agentctl reject my-workload --rule budget --reason "too many replicas"
agentctl edit-approve my-workload --edit-file edit.json --reason "smaller change"
```

`approve` and `reject` keep their legacy annotations and the Argo resume. They
add the protocol annotations only when a direct-path action is pending.

## Identity and the webhook

The approver is never taken from the client. The mutating webhook stamps
`approval-by` and `approval-for` from the admission request user. It rejects:

- a request that sets any stamp (`approval-by`, `approval-for`,
  `approval-mac`) itself,
- a change or removal of a decision already recorded for the pending action
  (decisions are append-once per pending action),
- a decision when no action is pending.

Enable it with `agenticOperator.webhook.agentWorkloadEnabled=true`. This also
turns on the existing validating rules, for example `https` MCP endpoints.

Before this change the AgentWorkload webhook registered no handler at all.
`SetupWebhookWithManager` now wires the defaulter and validator.

### Stamp key

The stamp is tamper-evident. The webhook adds `approval-mac`, an HMAC-SHA256
with a domain separator over:

- the workload UID,
- the pending action `id`,
- a digest of the decision, reason, and edit annotations,
- a digest of the `approval-by` stamp,
- a digest of the stored `status.pendingApproval.proposal`.

The key is a file named by `APPROVAL_STAMP_KEY_FILE`, at least 32 bytes. The
chart mounts it from a secret with `global.approvals.stampKey.existingSecret`
(key `stamp.key` by default):

```sh
kubectl create secret generic approval-stamp-key \
  --from-literal=stamp.key="$(openssl rand -hex 32)"
```

The controller recomputes the MAC from the live object and compares with
`hmac.Equal`. It does not act and sets condition `ApprovalDecision` to `False`
when:

| Reason | Cause |
|---|---|
| `MissingApprovalStamp` | No `approval-by` stamp |
| `ApprovalStampKeyMissing` | No usable key configured |
| `ApprovalStampInvalid` | MAC missing or wrong: forged stamp, decision changed after stamping, workload UID or pending proposal changed |

Approvals fail closed. With no key the webhook rejects new decisions and the
controller refuses all of them. With the webhook disabled nobody can mint a
valid MAC, so a patcher cannot forge an approval even with the right
`approval-by` JSON. Run the webhook and set the key in any cluster where
approvals matter.

### Trust boundary

The MAC stops annotation forgery. It does not protect `status`. A principal
with `update` or `patch` on `agentworkloads/status` can rewrite the pending
action, its proposal, and the consumed id list. Treat such principals as
inside the trust boundary unless a status admission policy is also enforced.
Only these subjects should hold that verb:

- the operator manager service account (`manager-role`),
- cluster admins (`cluster-admin`).

Do not grant `agentworkloads/status` to agents or CI bots. Today the
`agentctl-web` service account (`charts/templates/webui.yaml`) holds `patch`
and `update` on it, so the web UI is inside the trust boundary. Do not expose
it to untrusted users.

## What the controller does

On a stamped decision for the current pending `id`:

1. Approve and edit re-run invariants, policy packs, and the threshold on the
   action that would execute. Edit runs them on the edited action.
2. It records the decision in `status.pendingApproval.state` (`Recording`).
3. It writes a layer `human` receipt: outcome `approved`, `rejected`, or
   `edited`. The receipt `human_principal` is the approver identity digest
   (see [receipts](receipts.md)), never the raw username. Reasons hold the
   same digest, `sha256:` of the reason, and the pending `id`. The raw
   username appears only in the `ApprovalDecision` condition message, so an
   operator can see which RBAC subject decided.
4. With `RECEIPTS_REQUIRED=true` and a failed write, it stops. Condition
   reason `ReceiptFailed`. The action stays pending.
5. It appends the dataset example.
6. Then it acts:
   - `reject`: phase `Rejected`. Never executes. Terminal for the direct path.
   - `approve` or `edit` blocked by an invariant, a pack deny, or a pack
     error: a deny receipt follows, phase `PolicyDenied`. A human resolves
     threshold and pack approval findings. Never invariants.
   - otherwise: state `Executing`, then `execute_action`, then `Completed` or
     `Failed` as on the allow path.

Pending ids do not repeat. With receipts on, the id is derived from the
decision receipt entry hash and the workload UID, so a new pending action
always gets a new id and an old stamped decision is stale for it.

A decided id is consumed. The controller keeps the last 32 decided ids in
`status.consumedApprovalIDs`. The human receipt records
`consumed_id_sha256`, a domain-separated digest of the workload UID and
pending id, and `pending_payload_sha256`, the digest of the stored proposal
the decision was stamped against. If a status write restores a pending action
whose id is consumed, the controller does not act. Condition reason
`ApprovalReplay`.

The stamp MAC binds the decision to the workload UID and the stored proposal
digest. A decision copied to a recreated workload, or a proposal rewritten
after stamping, gives reason `ApprovalStampInvalid`.

`status.lastApproval` records the outcome: `executed`, `failed`, `rejected`,
`denied`, or `unknown`.

Replays are safe. A stale cached object fails the `Recording` status update
before any side effect. If the operator stops while `Executing`, the next
reconcile does not run the action again. It marks `Failed` with outcome
`unknown`. A crash between the receipt write and the next status update can
write a second identical human receipt. It cannot execute twice.

## Capture modes

`spec.approvalCapture.content` controls what the dataset stores about the
action:

| Mode | Stores |
|---|---|
| `none` | `action_sha256` and detected class names only |
| `redacted` (default) | Name, description, and params. Any string holding a detected class or credential becomes `[class]` |
| `full` | Raw name, description, and params |

`full` may hold personal data. Choose it explicitly and treat the writer volume
as personal data storage. Map keys are not scanned in `redacted` mode.

Receipts and decision records never hold raw params, in any mode.

`status.pendingApproval.proposal` holds the raw MCP proposal while an action
waits, because `approve` must execute it unchanged. Anyone who can read the
workload can read it.

## Dataset

The receipt-writer owns `approvals.jsonl` in its data dir. Routes, same bearer
token as receipts: `POST /v1/approvals` (strict decode, 256 KiB cap, unknown
fields and trailing data rejected) and `GET /v1/approvals`. The
writer rejects an example that does not bind to a stored human receipt (422)
and a second example for the same receipt or pending id (409).

One example per line:

| Field | Meaning |
|---|---|
| `schema_version` | `1` |
| `example_id` | Pending action `id` |
| `receipt_seq`, `receipt_entry_hash` | The human decision receipt |
| `workload` | Namespace, name, uid |
| `features` | Copied from the original decision record: original seq and input hash, declared decision type and allow lists, observed destination host, data classes, tool, history counts, claimed confidence and health (decimal strings), packs, threshold mode, `escalated_by` layer, reason codes |
| `label` | `approve`, `reject`, or `edit` |
| `reason_sha256`, `approver_sha256` | Hashes only |
| `timestamp` | RFC 3339 UTC |
| `capture_mode`, `content` | Per the capture mode |
| `edit_diff` | For edit: fields changed (`name`, `description`, `params`) and the edited content per mode |
| `source` | Absent for a human decision. `synthetic` marks generated training rows from `tools/decision-train`. Synthetic rows never bind to a receipt, so verify rejects them |

A failed dataset write is logged and shown in condition `ApprovalDataset`. It
does not block the decision.

## Export and verify

```sh
agentctl dataset export --writer http://localhost:8080 \
  --token-file ./token --out ./dataset
agentctl dataset verify ./dataset --trust-root ./pinned-trust.json
```

`verify` requires `--trust-root`, a trust file pinned out of band. The export's own
`trust.json` is not trusted, because anyone who can replace the export can replace
it too. `--allow-embedded-trust` runs a consistency-only check and prints a
WARNING. `verify` runs the same chain and record checks as `agentctl receipts verify`,
including the signed manifest completeness check. `--allow-prefix` works the
same way and prints the same WARNING.
Then for each example it checks that `receipt_seq` and `receipt_entry_hash`
exist in the chain, that the receipt is a `human` record, and that the label,
pending id, approver hash, reason hash, and workload match it. It prints PASS
or FAIL lines and exits 1 on any failure.

## Gaps

- Orchestrated (Argo) workloads: the approval gate is Argo `suspend`, resumed
  by `agentctl approve`. That path has no stamped identity, no human receipt,
  and no dataset example. Not changed here.
- `agentctl-web` approvals are stamped with the web service account, not the
  signed-in user.
- Receipts off means no human receipt and no dataset example. Decisions still
  work.
- Lost examples: if the dataset write fails, the label is not retried.
- `Rejected` is terminal. Recreate the workload to retry.
- `POST /v1/approvals` binding check scans the chain files. Fine for now, slow
  for very long chains.
- No synthetic seed set yet. The dataset starts empty.
- Dataset examples are bound to receipts by identity and label only. Feature edits and deleted rows in an export pass `dataset verify` (BLOCKERS.md, F7 and F12).

## Spec is frozen while an approval is pending

The stamp binds a decision to the stored proposal, not to the spec. So the
webhook rejects changes to `spec.declaredIntent`, `spec.policyPacks`,
`spec.opaPolicy`, `spec.decisionModel` and `spec.approvalCapture` while an
action is pending or a decision is being recorded. `spec.mcpServerEndpoint` is
always immutable. Approve, reject or edit the pending action first, then change
the spec.
