# Blockers

Open items that need a founder action before this branch can ship.

## receipt-writer image

The chart `receipts` block points at
`ghcr.io/clawdlinux/agentic-operator-core/receipt-writer`. No CI job publishes
that image yet. Build it with `docker build --target receipt-writer .` and push
it, or add a release job.

## Demo-only image

`scripts/demo-claims.sh` still builds linux binaries on the host and packages
them with `scripts/demo/Dockerfile.demo`. The repo Dockerfile now builds
(agentgate v0.1.4 is pinned). Switch the demo to the repo Dockerfile targets
and delete `Dockerfile.demo` when convenient.

## Known review findings

An independent review of this branch found these gaps. They are documented, not fixed.

- **F7. Dataset content is not bound.** `pkg/dataset/dataset.go` (`Validate`) checks identity and label fields and the original seq. It does not bind features, content hashes, captured params, timestamps or edit content. Someone who can edit an export can poison training features and `agentctl dataset verify` still passes. Fix: bind each example to its verified original decision record and rebuild features from it.
- **F11. Writer recovery trusts local files.** `pkg/receipts/local.go` checks hashes and the head KID on restart. It does not verify signatures or keep a committed-head checkpoint. Removing the tail of the chain rewinds the sequence and the writer can fork an already issued chain. Fix: verify signatures on recovery and persist an external checkpoint of the committed head.
- **F12. Deleted approval rows go unnoticed.** `pkg/dataset/verify.go` checks the examples it is given. Without a signed dataset manifest (count, order, digest) it cannot tell that rows were removed. Fix: sign a dataset manifest and verify completeness against it.

## Known gaps (not blockers for this branch)

- Runtime-adapter parity: `reconcileViaRuntime` runs no invariants, packs,
  receipts, or approvals. Packs on that path fail closed.
- Argo approval gates are not covered by human approvals.
- Execution outcomes are not receipted. Only the pre-execution decision is.
- Chart egress: the default-deny NetworkPolicy has no rule for the workload
  MCP endpoint. On an enforcing CNI (kindnet included) set
  `networkPolicy.additionalAllowedHosts`.
- A Completed direct-path workload is reconciled every 30s and proposes and
  runs a new action each time. Pre-existing behavior, unchanged here.
- `callerIdentity` is always empty.
- `TestControllers` needs envtest binaries (`bin/k8s` or
  `/usr/local/kubebuilder/bin`). It fails locally without them.
- **Denied direct-path workloads are not terminal.** A `PolicyDenied` or `Completed` direct-path workload is proposed again on any later reconcile (stale event, hourly or 30s requeue). Each proposal is fully re-decided and receipted, so no guard is skipped. But a deny caused by a temporary condition, such as an unreachable receipt writer under INV-05, can be followed by an execution seconds later. The demo hit this once. This predates the branch. Fix: make terminal phases sticky until the spec generation changes.
