# Invariants

Invariants are the deterministic floor of the
[decision architecture](decision-architecture.md). They live in
`pkg/invariants`. Pure Go. No model, no network. Each has a stable ID.

An invariant finding always denies. Policy packs cannot override it. Agent
claims cannot override it. The threshold evaluator cannot override it. In
strict mode the workload goes to `PolicyDenied`. In permissive or unset mode it
goes to `PendingApproval`. The action is never executed by the controller.

The set stays under 15 entries. IDs are never reused or renumbered.

| ID | Blocks | Applies when | Input trust class |
|---|---|---|---|
| INV-01 | Credentials reaching the agent. A credential-shaped value (AWS access key ID, PEM private key, JWT, `Bearer` token, GitHub `ghp_` or `github_pat_` token) in the proposed action. | Always, with or without `declaredIntent`. Declaring `credential` as allowed does not lift it. | Observed (`pkg/dataclass` class `credential`) |
| INV-02 | Egress to a destination not in `allowedDestinations`. | `declaredIntent` has any field set. An empty allow list allows nothing. | Declared vs observed |
| INV-03 | Personal data (`email`, `phone`, `aadhaar`, `pan`, `iban`, `card`) sent to a destination not in `allowedDestinations`. | `declaredIntent` has any field set. | Declared vs observed |
| INV-04 | Observed data classes outside `allowedDataClasses`. | `declaredIntent` has any field set. | Declared vs observed |
| INV-05 | Deciding without a receipt when receipts are required. Fails closed if no receipt writer is available or the receipt append fails. | `RECEIPTS_REQUIRED=true`. Off by default. See [receipts](../receipts.md). | Platform |
| INV-06 | Acting on a payload the data class scan could not fully read. Depth 8, 64 KiB per string, 1 MiB and 10000 nodes per value, or a leaf of an unknown type. Unscanned content could hold a credential or personal data, so the action is denied. | Always. | Observed (`dataclass.Scan` completeness) |

Reasons never contain matched content. They name the class or host only.

## Where invariants run

- Legacy direct-action path: every action, before packs and thresholds.
- Runtime-adapter path (`spec.orchestration`): not yet. This is a known gap.

## Known limits

- Detection is pattern based. A credential in an unknown format is missed.
- Scan limits are kept. A payload past them is denied by INV-06, never
  partly scanned and allowed.
- Destination is the configured MCP endpoint, not every host the MCP server
  calls. See [policy input](../policy-input.md).
- INV-01 only sees the `propose_action` reply. The objective was already sent
  to the MCP server before any check runs.
