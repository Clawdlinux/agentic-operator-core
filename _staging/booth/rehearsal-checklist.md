# Booth rehearsal checklist

Run this flow 5 times on the booth Mac. The real provider steps require network
access. Keep the prior-run verifier available when venue network access fails.

Driver script: `scripts/demo-booth.sh`.

## 0. One-time prep

- [ ] Install `kubectl`, `helm`, `kind`, Docker, `curl`, and Go.
- [ ] Build the current operator image using the configured demo image tag.
- [ ] Build `bin/agentctl` with `make build-agentctl`.
- [ ] Build `bin/audit-verify` with `make obs-build-verifier`.
- [ ] Confirm `_staging/booth/attestation-fallback.jsonl` verifies offline.
- [ ] Keep the repo-root `.env` ignored and readable only by the booth operator.

## 1. Prepare with real credentials

- [ ] Export both provider keys or define them in the repo-root `.env` file.
- [ ] Set `DEMO_ENV_FILE` when the credential file lives elsewhere.
- [ ] Run `scripts/demo-booth.sh --prepare`.
- [ ] Confirm both variable names report `available` without showing values.
- [ ] Confirm the reported source is `environment` or the expected file.
- [ ] Confirm exported `LITELLM_MASTER_KEY` values with LF or CR are rejected.
- [ ] Confirm context is `kind-clawdlinux-demo`.
- [ ] Confirm cert-manager, webhook, operator, and LiteLLM become ready.
- [ ] Confirm the script never prints key values.
- [ ] Unset exported provider variables after preparation on shared terminals.

## 2. Present the 5-7 minute flow

- [ ] Run `scripts/demo-booth.sh --present`.
- [ ] **REAL:** AgentWorkload reaches `Completed` after one OpenAI-routed call.
- [ ] **REAL:** Routing condition shows genuine input and output token counts.
- [ ] **REAL:** Cost annotation is greater than zero.
- [ ] **REAL:** `clawdlinux_agent_cost_dollars` is greater than zero.
- [ ] **REAL:** Separate Anthropic request reports provider reachability.
- [ ] Do not claim the AgentWorkload used Anthropic.
- [ ] **SIMULATION / CONFIGURATION PROOF:** Dry-run injects `runtimeClassName=gvisor`.
- [ ] State that no gVisor pod was scheduled on kind/macOS.
- [ ] **NETWORKPOLICY OBJECT PRESENCE ONLY:** Show the NetworkPolicy object.
- [ ] Say packet enforcement requires an enforcing CNI.
- [ ] Do not claim all runtime pods carry sandbox labels.
- [ ] State that `--present` does not execute OPA allow or deny evaluation.

## 3. Prior-run audit closer

- [ ] State that the current workload did not create the fixture.
- [ ] **PRIOR-RUN ARTIFACT:** Verify the checked-in JSONL file offline.
- [ ] Optionally run `scripts/demo-booth.sh --present --tamper-audit`.
- [ ] Confirm the altered temporary artifact fails verification.
- [ ] State that audit recording is not wired into reconciliation yet.

## 11. Teardown

- [ ] `scripts/demo-booth.sh --cleanup` deletes the kind cluster.
- [ ] Confirm `kind get clusters` no longer lists `clawdlinux-demo`.

## Rehearsal log

| Run | Date | Network stable? | All gates pass? | First failure | Total time | Notes |
|-----|------|-----------------|-----------------|---------------|------------|-------|
| 1   |      |                 |                 |               |            |       |
| 2   |      |                 |                 |               |            |       |
| 3   |      |                 |                 |               |            |       |
| 4   |      |                 |                 |               |            |       |
| 5   |      |                 |                 |               |            |       |

Booth-ready bar: 5 consecutive runs under 7 minutes with every truth label spoken.

## Optional: record a backup

- [ ] `scripts/record-demo.sh` records the present path.
- [ ] Set `DEMO_TAMPER_AUDIT=true` to include the tamper proof.
- [ ] Review the recording for accidental key output before sharing it.
