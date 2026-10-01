# Blockers

Open items that need a founder action before this branch can ship.

## receiptspec dependency

`go.mod` requires `github.com/Clawdlinux/agentgate v0.1.3` with a local
`replace` to `../agentgate-receiptspec`. That directory only exists on the
founder laptop. `pkg/receipts`, `cmd/receipt-writer`, and `agentctl receipts`
import `github.com/Clawdlinux/agentgate/pkg/receiptspec`, which is not in any
pushed or tagged AgentGate release yet.

Until this is fixed, CI and `docker build` fail. Both run without the sibling
directory, so the `replace` target is missing.

Fix, in order:

```sh
# 1. AgentGate: push, merge, tag.
cd ../agentgate-receiptspec
git push -u origin feat/receiptspec
gh pr create --base main --head feat/receiptspec --fill
gh pr merge --squash --delete-branch
git fetch origin && git checkout main && git pull --ff-only
git tag -a v0.1.4 -m "v0.1.4: public pkg/receiptspec" && git push origin v0.1.4

# 2. Operator: drop the replace and pin the tag.
cd ../operator-decision
go mod edit -dropreplace=github.com/Clawdlinux/agentgate
go mod edit -require=github.com/Clawdlinux/agentgate@v0.1.4
go mod tidy
go build ./... && go test ./pkg/receipts/... ./cmd/receipt-writer/... ./cmd/agentctl/...
git commit -s -am "chore(deps): pin agentgate v0.1.4, drop local replace"
```

Then delete this entry.

## receipt-writer image

The chart `receipts` block points at
`ghcr.io/clawdlinux/agentic-operator-core/receipt-writer`. No CI job publishes
that image yet. Build it with `docker build --target receipt-writer .` and push
it, or add a release job. Blocked on the receiptspec entry above.

## Demo-only image

`scripts/demo-claims.sh` builds linux binaries on the host and packages them
with `scripts/demo/Dockerfile.demo`, because the repo Dockerfile cannot build
while the receiptspec `replace` is in `go.mod`. Once v0.1.4 is pinned, switch
the demo to the repo Dockerfile targets and delete `Dockerfile.demo`.

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
