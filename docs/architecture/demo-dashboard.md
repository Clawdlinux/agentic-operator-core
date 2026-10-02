# Demo dashboard

Status: iteration 1 shipped. The summary tiles landed on `/decisions`, not on `/`. The existing `/` workload dashboard is unchanged. Follow-ups: per-namespace filtering, a paged export from the writer, an approvals queue with edit on these pages.

## Problem

The operator now makes layered decisions and signs a receipt for each one. A buyer cannot see any of it without a terminal. `agentctl-web` exists, but it shows workloads, cost and status. It shows nothing from the decision architecture.

## What exists today

- `cmd/agentctl-web`: Go, `html/template`, vendored htmx, no external assets. Token auth through TokenReview and SubjectAccessReview. CSRF and audit middleware. Chart block `webUI`, default off.
- Workload list, describe, approve and reject. Approvals are stamped with the web service account (`docs/approvals.md`).
- A demo page that runs without a cluster (`--demo`).
- The receipt-writer serves `GET /v1/export`, `/v1/head` and `/v1/approvals` behind a bearer token. `pkg/receipts.ReadExport` and `VerifyExport` read and verify an export.

## Goal

One screen flow that shows, from real receipts, what the operator decided and why. It must run in two places.

1. Offline from an export directory. No cluster, no network. This is the booth path.
2. In the cluster, next to the operator, reading the receipt-writer.

## Non-goals

- No hosted service, no telemetry, no external fonts or scripts. The ADR keeps the plane air-gapped.
- No write path beyond the approve and reject buttons that already exist.
- No new data store. The receipt chain is the source of truth.
- No claim about the runtime-adapter path or execution outcomes. They are not receipted. The UI says so.

## Views

| Path | Shows | Source |
|------|-------|--------|
| `/decisions` | Summary tiles, chain status, a fixed "not covered" panel, and the timeline. Seq, time, workload, action, deciding layer, outcome, reason codes. Filter by outcome. | records in the export |
| `/decisions/{seq}` | One decision. Layers and reasons, observed data classes and host, packs, threshold mode, model block (mode, risk, threshold, base and final outcome), approval block, chain link and signer. | one record and its receipt |
| `/receipts` | Chain head, count, signer, verification result with the reason on failure, completeness against the signed manifest, link to download the export. | `VerifyExport` |


Verification runs in the web process against a pinned trust root. The page never trusts the export's own `trust.json`. Without a pinned root the page says "consistency only" and shows no PASS badge. This matches `agentctl dataset verify`.

Privacy matches the receipts. Records hold hashes and class names, not raw params. The UI shows only what the record holds. All text goes through `safeText` and the template escaper.

## Design

- `pkg/decisionview`: loads an `Export` from a directory or the writer, verifies it, and builds view models. No HTTP, no templates. Table-driven tests with the existing export fixtures.
- `cmd/agentctl-web`: new handlers and templates. Flags `--export-dir`, `--writer-url`, `--writer-token-file`, `--trust-root`. The offline mode needs no kubeconfig and no auth, and binds to localhost unless told otherwise.
- Chart `webUI.receipts`: writer URL, token secret, trust root ConfigMap. A NetworkPolicy rule lets the web pod reach the writer. The writer ingress rule admits the web pod.
- Refresh: htmx polls the summary every 5 seconds. Reads are cached for 2 seconds to protect the writer.

## Loop

Each step is small, verified and committed on its own. Stop when every done check passes.

1. Data layer `pkg/decisionview`. Done when tests cover timeline, detail, chain failure and unpinned mode.
2. Handlers and templates for `/decisions`, `/decisions/{seq}`, `/receipts`. Done when handler tests assert content, escaping and no external host in any page.
3. Summary tiles and the "not covered" panel on `/decisions`.
4. Chart wiring and `helm unittest` cases.
5. Offline demo: `make demo-dashboard` serves a fixture export. Screenshot check with Playwright at 375 and 1280 px.
6. Docs: `docs/dashboard.md`, CHANGELOG, BLOCKERS.

## Done checks

- `go build`, `go vet`, `make lint`, `go test` for the touched packages.
- `helm unittest charts`.
- Offline demo shows PASS for a pinned root and a visible FAIL after one byte of a record changes.
- No request leaves localhost in offline mode.
- Copilot review has no open finding.

## Risks

- The writer can be slow or down. The page shows the error and the last good data age. It never shows stale data as fresh.
- A large chain. Paginate the timeline at 100 rows. Verify once per refresh, not per request.
- Pending approvals live in workload status, not in the receipt chain. The approvals view keeps using the existing handlers.
