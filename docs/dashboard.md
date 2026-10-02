# Decision dashboard

The dashboard shows what the operator decided and whether you can trust it. It reads signed receipts. It runs offline from an export, or in the cluster next to the operator.

## Run it offline

No cluster and no network.

```sh
make demo-dashboard
```

This writes a sample export with a fixed demo key and serves it on `http://127.0.0.1:8090`. The sample has allowed actions, a deny by an invariant, a deny by a policy pack, an approved and a rejected escalation, one decision still waiting for a human, and a shadow-mode model score.

To serve your own export:

```sh
agentctl receipts export --writer http://localhost:8080 --token-file ./token --out ./export
agentctl-web --export-dir ./export --trust-root ./pinned-trust.json
```

Offline mode binds to localhost unless you pass `--addr`. It has no login. Do not expose it.

## Pages

| Path | Shows |
|------|-------|
| `/decisions` | Chain status, counts, and a timeline of decisions, newest first. Filter by outcome. Refreshes every 5 seconds. |
| `/decisions/{seq}` | One decision: the deciding layer, each finding, what the platform observed, what was declared, what the agent claimed, the model block, a human decision, and the receipt hashes. |
| `/receipts` | Chain head, receipts verified, signed manifest, completeness, and whether the trust root is pinned. |

The chain status block shows on every page. The "Not covered" list shows on `/decisions` and `/receipts`. It names what receipts do not cover: the runtime-adapter path, execution outcomes, Argo approval gates, and the live approval queue.

## What the badge means

The dashboard verifies the chain itself. It does not take the export's word.

| Badge | Meaning |
|-------|---------|
| Verified | Signatures check out under a pinned key and a signed manifest proves nothing was removed. |
| Partial | Signatures check out under a pinned key. Nothing proves later receipts were not removed. |
| Consistent only | The chain is consistent, but the trust root came from the export itself. Nobody is authenticated. |
| Failed | A signature, a link, or a record does not check out. The rows are not trustworthy. |

Pin a trust root with `--trust-root`. Without it the badge never says Verified. This is the same rule as `agentctl dataset verify`.

## Run it in the cluster

Turn on the web UI and the receipt-writer together:

```sh
helm upgrade --install agentic-operator charts \
  --set webUI.enabled=true \
  --set global.receipts.enabled=true \
  --set global.receipts.signingKey.existingSecret=receipt-signing-key \
  --set global.receipts.trustRoot.existingConfigMap=receipt-trust-root
```

The chart passes the writer URL, the token file and the pinned trust root to `agentctl-web`. It also lets the web pods reach the writer through its NetworkPolicy. The Decisions and Receipts links appear in the navigation. Pages sit behind the same token login as the rest of the web UI.

Without `global.receipts.trustRoot.existingConfigMap` the pages work, but the badge reads Consistent only.

## What it does not do

- It reads receipts. It does not write them. Approve and reject still use the workload pages.
- The live approval queue is the workload status, not the chain. A held decision already has a `require_approval` receipt, so it shows on the timeline as awaiting approval. The human decision joins the chain later.
- Decision records hold hashes, class names and a host. The pages show nothing more, so they cannot show raw payloads.
- Any signed-in user who can reach the pages sees every decision. There is no per-namespace filter yet.

## Load and size

The page asks the writer for a full export. It caches the result for 2 seconds and keeps the last good result if a refresh fails, with a visible notice. The timeline shows 100 rows at a time. Very long chains need a paged export from the writer. That is not built.
