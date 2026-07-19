# Agentic Summit: Claude and ANF Booth Walkthrough

## Decision

Run a 5-minute live demo at the booth. Keep the control room visible throughout.

Use 2 optional bookend slides only for scheduled conversations. The live proof remains primary.

## Before the booth opens

Run preparation once after rebuilding the operator image. Do not prepare during a visitor conversation.

- [ ] Verify the kind context: `test "$(kubectl config current-context)" = "kind-clawdlinux-demo" && echo "context ok"`
- [ ] Verify pods are ready: `kubectl -n agentic-system wait --for=condition=Ready pod --all --timeout=60s`
- [ ] Build the booth tools: `make build-agentctl build-anf-snapshot`
- [ ] Run `./scripts/demo-booth.sh --prepare` after loading the current operator image.
- [ ] Keep `http://127.0.0.1:8765` closed until the visualizer starts its countdown.
- [ ] During the countdown, open that URL and make the browser full-screen.
- [ ] Mute notifications, close secrets, and keep one terminal ready at the repository root.
- [ ] Record one local end-to-end run with `scripts/record-demo.sh --scenario all --pace 3 --duration 120 --no-open`.
- [ ] Know the fallback log path: `demos/local/latest.log`.
- [ ] Keep `demos/local/latest.webm` available as a no-server fallback.

These commands do not print credential values.

## Opening narrative: problem, market, and solution

Keep this opening under 60 seconds. It replaces a separate opening slide during normal booth conversations.

### Problem statement

AI agents already call models, tools, and infrastructure. The deployment blocker is trust.
Platform and security teams need to know what ran and which controls applied.
They also need cost data and evidence that exposes later changes.

Existing controls answer separate questions. A sandbox constrains the process. A NetworkPolicy declares traffic rules.
A model gateway records usage. An audit log records events.
Teams still lack one reviewable record connecting those layers to a governed agent workload.

### Market evidence

- Gartner reported in June 2025 that more than 40% of agentic AI projects will be canceled by the end of 2027. It named escalating cost, unclear value, and inadequate risk controls among the causes.
- AIRQ reported in June 2026 that only 11% of 100 assessed production agents passed its security bar.
- A US bank self-reported in May 2026 after customer data was sent to an unauthorized AI application.

These sources validate the problem category. They do not validate Clawdlinux product demand.
Clawdlinux is pre-revenue and has no customer or production-adoption claim today.
The next validation step is a design-partner pilot with a regulated platform and security team.

Sources are tracked in [MARKET-EVIDENCE.md](MARKET-EVIDENCE.md).

### Solution

Clawdlinux is an in-cluster governance and evidence plane around Kubernetes agent runtimes. It does not replace the agent framework or runtime.

The target product connects a reviewable workload contract, runtime isolation, egress controls, model usage attribution, and tamper-evident receipts. Today's demo proves a narrower slice. Every panel states whether its evidence is `LIVE`, `CONFIG ONLY`, or `PRIOR RUN`.

## Five-minute run of show

### 0:00-0:50. Problem, market signal, and solution

**Operator action:** Share the ready terminal. Keep the command visible but unstarted.

**Screen state expected:** The repository root is visible. No secret files or environment values are visible.

**Exact words to say:**

"AI agents can already call models, tools, and infrastructure. The hard part is getting them through a security review.
Gartner reported that more than 40 percent of agentic AI projects will be canceled by the end of 2027.
Its cited causes include cost, unclear value, and inadequate risk controls.
AIRQ reported that only 11 percent of 100 assessed production agents passed its security bar.

Clawdlinux is an in-cluster governance and evidence plane around Kubernetes agent runtimes. It does not replace the runtime. It connects workload state, control configuration, model usage, and tamper-evident evidence. We are pre-revenue and have no customer claim today. This validates technical proof, not market demand. I will label what is live, configuration-only, and from a prior run."

### 0:50-1:05. Start the live view

**Operator action:** Run this command:

```bash
python3 scripts/demo-visualizer.py --present --scenario all --tamper-audit --pace 6
```

Open `http://127.0.0.1:8765` during the countdown. Make it full-screen.

**Screen state expected:** The branded control room loads. Panels show pending states before evidence arrives.

**Exact words to say:**

"I started one command. This dashboard stays connected while success and controlled-fault YAML run in sequence. The terminal remains the source stream."

### 1:05-2:30. Live Kubernetes, ANF, workload, and Claude

**Operator action:** Follow the 5 verified nodes from left to right.

**Screen state expected:** Kubernetes, ANF, AgentWorkload, LiteLLM, and Claude become verified.

**Exact words to say:**

"The selected fixture and workload paths are visible above the pipeline. Platform teams can review or edit those YAML files before a run.

The success fixture exits cleanly. The controlled-fault fixture exits with an invalid flag. Each observed result must match its declared expectation before analysis starts.

Clawdlinux captures live Kubernetes state after each fixture reaches its terminal state. It projects that state into ANF and reports omitted facts separately.

The script injects this ANF into the scenario AgentWorkload. In-cluster LiteLLM routes analysis to Claude Haiku 4.5. Completion, token counts, and estimated cost come from the live workload."

### 2:30-3:10. Configuration evidence and limits

**Operator action:** Point to CONFIG ONLY. Read both control states without expanding their claims.

**Screen state expected:** gVisor says dry-run injected with no pod scheduled. NetworkPolicy says object present with no packet test.

**Exact words to say:**

"These are configuration proofs, not runtime enforcement proofs. A server-side dry-run shows the webhook injecting runtimeClassName gvisor. No pod was scheduled. The NetworkPolicy object exists. We did not test packet enforcement, which also depends on the cluster CNI."

### 3:10-4:05. Prior-run audit and tamper rejection

**Operator action:** Point to PRIOR RUN. Show the verified receipt, then the tamper rejection.

**Screen state expected:** The prior-run HMAC fixture passes. A temporary modified copy is rejected. The event tail shows both results.

**Exact words to say:**

"This receipt is a prior-run HMAC audit fixture. The current workload did not generate it. First, the verifier accepts the unchanged artifact. Then the demo modifies a temporary copy without recomputing its MAC. Verification rejects that copy. Anyone with this committed demo key could re-sign a modified fixture, so this is not production key custody."

### 4:05-4:40. Product and current proof

**Operator action:** Hold on the 3 evidence panels. Do not navigate away.

**Screen state expected:** LIVE, CONFIG ONLY, and PRIOR RUN remain visible with the 5-line event tail.

**Exact words to say:**

"The product is the in-cluster governance and attestation layer around agent runtimes. Today's proof shows live Kubernetes context becoming ANF, Claude routing, measured usage, admission mutation, policy objects, and offline tamper detection. Full runtime enforcement and same-run signed attestation remain the next proof steps."

### 4:40-5:00. Closing ask

**Operator action:** Keep the completed control room visible. Stop moving the pointer.

**Screen state expected:** The final evidence remains readable. Do not replace it with a slide.

**Exact words to say:**

"We are looking for regulated Kubernetes teams with a blocked agent project. The design-partner offer is one use case, one cluster, and a 30-day pilot."

## Exact 90-second fallback rule

Start a 90-second timer when the live command begins.

If the workload is not Completed by 90 seconds, say this exact sentence:

"The live workload has not completed within my 90-second limit, so I am switching visibly to a recorded successful rehearsal."

Press `Ctrl+C` in the live terminal. Then run:

```bash
DEMO_REPLAY_DELAY_SECONDS=0.5 \
	python3 scripts/demo-visualizer.py --replay demos/local/latest.log
```

Open or refresh `http://127.0.0.1:8765` during its countdown. Confirm the screen says RECORDED REHEARSAL.

Say: "This is a recorded successful rehearsal, not a live run. I will continue from the same panel."

Resume at the panel where the live run stopped. Never call the replay live.

If the browser fails, return to the terminal. Say: "The terminal is the source stream, so I will continue there."

If the replay server fails, open `demos/local/latest.webm` full-screen.
Say: "This is a recording of a successful live run. It is not the current live run."

Never swap to replay silently.

## Questions likely at the booth

### Is Claude the real AgentWorkload route?

"Yes. Every task category in this booth workload maps to the Claude alias through in-cluster LiteLLM."

### Is the ANF reduction fair?

"Yes. The dashboard compares the same projected `anf.Document` encoded as JSON and ANF. Omitted Kubernetes facts are counted separately."

### Is gVisor really running?

"Not in this path. Server-side dry-run proves mutation to runtimeClassName gvisor. No pod is scheduled by that check."

### Is the network enforced?

"This demo proves the NetworkPolicy object exists. It does not run a packet test or prove CNI enforcement."

### Is the audit from this run?

"No. It is a prior-run HMAC fixture. The demo verifies it, modifies a temporary copy, and proves rejection."

### Who buys this?

"Platform and security teams running AI agents in regulated or isolated Kubernetes environments are the intended buyers. We have no customer claim today."

### How is this different from gVisor or NetworkPolicy alone?

"Those controls isolate workloads or traffic. Clawdlinux connects runtime governance with model usage evidence and tamper-evident audit receipts."

### What works today versus the target?

"Today, model routing, usage evidence, admission mutation, policy objects, and prior-run verification work. Same-run attestation and enforcement tests are next."

## Optional bookend slides

Use these only for scheduled conversations. Do not use them for normal booth traffic.

### Slide 1. Problem

**One sentence:** AI agent actions need evidence that remains trustworthy after execution.

- Agents cross model, tool, and runtime boundaries.
- Existing controls prove separate layers, not the full action trail.
- Clawdlinux joins governance with tamper-evident receipts.

### Slide 2. Proof and ask

- Proof: real routing, tokens, cost, control configuration, and tamper rejection.
- Ask: identify one regulated team with a blocked Kubernetes agent project.
- GitHub: `https://github.com/Clawdlinux/agentic-operator-core`

Protect the closing slide. Under time pressure, cut the opener slide first.

## Do not say

- Do not say ANF compresses omitted Kubernetes facts.
- Do not say the product requires Claude or LiteLLM.
- Do not say gVisor ran a pod in this path.
- Do not say NetworkPolicy packet enforcement was tested.
- Do not say the audit fixture came from the current workload.
- Do not say replay is live.
- Do not claim same-run signed attestation is complete.
- Do not claim customers, revenue, deployments, or production adoption.
- Do not mention OPA unless someone asks about it.
- Do not call configuration evidence runtime proof.

## One-line close

"Clawdlinux makes agent actions governed, measured, and independently verifiable inside the cluster."