# Booth rehearsal checklist

Cold-run gate for the Clawdlinux attestation demo. Run this 5 times on the actual
booth laptop, offline, before the summit. Assume venue wifi is dead. Tick every
gate. If any gate fails twice, the demo is not booth-ready.

Driver script: `scripts/demo-booth.sh` (560 lines). This checklist mirrors its
phases and adds explicit pass/fail gates the script does not assert for you.

## 0. One-time offline prep (do once, before rehearsals)

- [ ] Laptop has `kubectl`, `helm`, and `kind` (or `k3s`) installed and on PATH.
- [ ] Seed the kind node-image cache with the same command the demo script uses:
      `kind create cluster --name clawdlinux-demo --wait 120s`
      This pulls the exact `kindest/node` tag for your installed `kind` binary.
      Without this, wifi-off `kind create cluster` fails immediately.
- [ ] Pre-pull every image the demo needs, then point kind at the local cache so
      no phase pulls from the network:
      - [ ] Keep `clawdlinux-demo` running while you load images.
      - [ ] For each image the chart and samples use: `docker pull <img>` then
            `kind load docker-image <img> --name clawdlinux-demo`.
      - [ ] Re-run the demo with wifi OFF. If any phase blocks on ImagePull, the
            cache is incomplete. Fix before the summit.
- [ ] `audit-verify` built and on PATH: `go build -o ~/bin/audit-verify ./cmd/audit-verify`.
- [ ] Fallback artifact staged: `_staging/booth/attestation-fallback.jsonl`
      (see `_staging/booth/README.md`). Confirm it still verifies offline.

## 1. Prerequisites (script: "Checking prerequisites")

- [ ] PASS gate: script prints "kubectl and helm found" and finds kind or k3s.
- [ ] FAIL signal: "kind or k3s is required" or a missing manifest. Stop, fix PATH
      or repo checkout.
- [ ] Confirm present: `config/samples/agentworkload_demo_{allow,deny,swarm}.yaml`.

## 2. Cluster prep (script: "Preparing Kubernetes cluster")

- [ ] PASS gate: "Created kind cluster clawdlinux-demo" (or "Reusing ...") within
      120s.
- [ ] FAIL signal: kind create hangs past 120s. Usually a stale Docker state.
      Run `scripts/demo-booth.sh --cleanup` and retry.

## 3. Operator ready (script: "Waiting for operator pod")

- [ ] PASS gate: operator pod reaches Running inside the 180s deadline.
- [ ] FAIL signal: deadline hit. Check `kubectl -n agentic-system describe pod`.
      Almost always an un-cached image. Go back to step 0.

## 4. Deployment profile (script: "Installing ... profile")

- [ ] Decide before you start: `--profile platform` (Argo and shared services)
      or `--profile lean` (operator and governance controls only).
      Rehearse the one you will present.
- [ ] PASS gate: helm install completes within `HELM_TIMEOUT` (default 180s).
- [ ] FAIL signal: helm timeout. Raise `HELM_TIMEOUT=300s` only if the laptop is
      slow; if it is an image pull, fix the cache, do not just raise the timeout.

## 5. Workload + runtime view (script: "AgentWorkload status", "Agent pod runtime view")

- [ ] PASS gate: the demo AgentWorkload shows a populated status; an agent pod is
      listed with its runtime view.

## 6. Cost metric (script: "Checking cost metric")

- [ ] PASS gate: a per-workload cost metric is reported (non-empty).
- [ ] This is the FinOps talking point. If empty, say so, do not improvise.

## 7. gVisor sandbox (script: "Checking gVisor RuntimeClass")

- [ ] PASS gate: the labeled pod shows `runtimeClassName: gvisor`.

## 8. Egress seal — the core demo (script: "Checking NetworkPolicy", "OPA allow", "OPA deny")

- [ ] PASS gate (allow): `agentworkload_demo_allow.yaml` runs; egress scoped to
      the FQDN allowlist.
- [ ] PASS gate (deny): `agentworkload_demo_deny.yaml` is blocked at the boundary.
- [ ] Talk track: "default deny, allowlist per workload, enforced at the network
      layer, not in app code."
- [ ] FAIL signal: deny case is NOT blocked. This breaks the whole pitch. Do not
      present until fixed.

## 9. Attestation artifact — the money shot

- [ ] During the run, capture the audit/attestation output to the evidence dir.
- [ ] Verify it offline in front of the prospect:
      `audit-verify --source jsonl --path <evidence>.jsonl --key <kid>=<b64>`
- [ ] PASS gate: "PASS — chain is intact".
- [ ] Tamper demo (optional, strong): edit one field, re-run audit-verify, show
      it FAIL with an entry_hash mismatch. Proves the artifact is real evidence.
- [ ] If the live run hiccups, fall back to `_staging/booth/attestation-fallback.jsonl`
      and run the same verify + tamper demo. Never debug live on a prospect's time.

## 10. Multi-agent swarm (optional scenario, needs `--with-swarm`)

- [ ] Only if presenting the swarm: run with `--with-swarm` and the platform profile.
- [ ] PASS gate: 3-agent swarm reaches a terminal state without manual nudging.

## 11. Teardown

- [ ] `scripts/demo-booth.sh --cleanup` deletes the kind cluster.
- [ ] Confirm `kind get clusters` no longer lists `clawdlinux-demo`.

## Rehearsal log (fill this in — 5 cold runs, wifi off)

| Run | Date | Wifi off? | All gates pass? | First failure (phase) | Time to green | Notes |
|-----|------|-----------|-----------------|-----------------------|---------------|-------|
| 1   |      |           |                 |                       |               |       |
| 2   |      |           |                 |                       |               |       |
| 3   |      |           |                 |                       |               |       |
| 4   |      |           |                 |                       |               |       |
| 5   |      |           |                 |                       |               |       |

Booth-ready bar: 5 consecutive cold runs, wifi off, all gates green, under your
demo time budget. Anything less and you are gambling on the venue.

## Optional: record a backup

- [ ] `scripts/demo-booth.sh --profile platform --with-swarm --record` captures terminal output with
      script(1). Keep one clean recording as a last-resort fallback if the laptop
      itself dies.
