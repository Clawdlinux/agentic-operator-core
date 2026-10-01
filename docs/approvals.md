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

- a request that sets either stamp itself,
- a change or removal of a decision already recorded for the pending action
  (decisions are append-once per pending action),
- a decision when no action is pending.

Enable it with `agenticOperator.webhook.agentWorkloadEnabled=true`. This also
turns on the existing validating rules, for example `https` MCP endpoints.

Before this change the AgentWorkload webhook registered no handler at all.
`SetupWebhookWithManager` now wires the defaulter and validator.

Without the webhook, the controller refuses any decision that has no
`approval-by` stamp. It sets condition `ApprovalDecision` to `False` with reason
`MissingApprovalStamp` and does not act. Without the webhook a client could
also forge the stamp. So run the webhook in any cluster where approvals matter.

## What the controller does

On a stamped decision for the current pending `id`:

1. Approve and edit re-run invariants, policy packs, and the threshold on the
   action that would execute. Edit runs them on the edited action.
2. It records the decision in `status.pendingApproval.state` (`Recording`).
3. It writes a layer `human` receipt: outcome `approved`, `rejected`, or
   `edited`. The receipt `human_principal` is the approver username. Reasons
   hold the approver, `sha256:` of the reason, and the pending `id`.
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

`verify` runs the same chain and record checks as `agentctl receipts verify`.
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
