# Demo Plan

One demo, 10 to 12 minutes, runs entirely on a local kind cluster via `scripts/demo-booth.sh`. Written for the co-founder walkthrough and the Jul 22 booth. Everything here is reproducible today; no step depends on unshipped code.

## The story we are telling

A regulated fintech platform team wants to run an analysis agent. Security will only sign off if three things are provable: the agent cannot do destructive work, cannot reach anything outside a declared allowlist, and every action is recorded in a way an auditor can verify offline later. We show all three, then break the audit trail on purpose and watch verification fail.

## Prep (before the call, not during)

```
./scripts/demo-booth.sh --profile platform  # kind + operator + Argo + shared services
./scripts/demo-booth.sh --cleanup  # to reset
```

Run it once the night before. It provisions the kind cluster (`ninevigil-demo`), installs the operator, Argo, and shared services, and leaves evidence under `tests/harness/evidence/`. Use `--profile lean` if time or laptop capacity is constrained. Keep a recorded run (`--record`) as backup in case live wifi or Docker misbehaves.

## Run of show

1. Frame (1 min). "Agent runtimes are getting good. What blocks production in a bank is governance: identity, blast radius, cost, and proof. We wrap any agent workload with those controls. Here is the whole thing on my laptop, air-gap friendly."

2. The manifest is the contract (2 min). Show `config/samples/agentworkload_demo_allow.yaml`. Point at the parts security cares about: `opaPolicy: strict`, the approval threshold, the declared endpoint. The review surface is a YAML file in a PR, not a prompt.

3. Policy allow vs deny (2 min). Apply the allow workload (`objective: analyze quarterly revenue data`), it proceeds. Apply the deny workload (`objective: delete all production volumes`), OPA rejects it. Same policy, no model involved in the decision.

4. Isolation and egress (2 min). Show the agent pod running under the gVisor RuntimeClass and the generated NetworkPolicy. The agent can only reach the FQDNs its manifest declared. This is enforced at the kernel and network layer, not by asking the model nicely.

5. Cost (1 min). Show the per-workload cost metric. Every run has an owner and a price.

6. The closer: tamper-evident audit (3 min). Show the audit chain entries for the run. Run `audit-verify` against a snapshot: PASS. Now edit one row in the snapshot and run it again: FAIL, chain broken at that seq. "This is what your auditor gets. They do not have to trust us or you. They recompute the chain offline."

7. Optional, if audience is technical and time allows (`--with-swarm`): the three-agent swarm (`agentworkload_demo_swarm.yaml`), scraper, synthesizer, report generator through an Argo DAG, same controls applied to a multi-agent run.

## Talk track guardrails

Do not name competitor runtimes as problems. Do not claim SPIFFE/multi-cluster, SOC 2, webhook validation, or an air-gapped CI gate; they are roadmap. If asked "does this work with runtime X": "the adapter interface covers our CRD, plain labeled pods, and external runtimes; the seal and attestation contract is the same for all three."

## Failure modes and fallbacks

Docker/kind flaky: play the recorded run, narrate live. Argo slow to schedule: use the lean profile, the policy and audit story does not need it. Question you cannot answer: "good question, that is exactly the kind of thing we gate the roadmap on, tell me more about your setup."
