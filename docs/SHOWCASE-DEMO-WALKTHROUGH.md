# India Builds with Claude: Screening Demo Walkthrough

## Decision

Run a 5-minute live demo for this screening. Do not use a deck during the screening.

Thursday may use 2 optional bookend slides. The product proof remains the live control room.

## Before the call (5 minutes)

Preparation is already done with `./scripts/demo-booth.sh --prepare`. Do not repeat preparation during the screening.

- [ ] Verify the kind context: `test "$(kubectl config current-context)" = "kind-clawdlinux-demo" && echo "context ok"`
- [ ] Verify pods are ready: `kubectl -n agentic-system wait --for=condition=Ready pod --all --timeout=60s`
- [ ] Build the repo-local CLI: `make build-agentctl`
- [ ] Keep `http://127.0.0.1:8765` closed until the visualizer starts its countdown.
- [ ] During the countdown, open that URL and make the browser full-screen.
- [ ] Mute notifications, close secrets, and keep one terminal ready at the repository root.
- [ ] Know the fallback log path: `demos/demo-20260713.log`

These commands do not print credential values.

## Five-minute run of show

### 0:00-0:35. Problem and value

**Operator action:** Share the ready terminal. Keep the command visible but unstarted.

**Screen state expected:** The repository root is visible. No secret files or environment values are visible.

**Exact words to say:**

"This is a screening, not the final showcase. AI agents can call models and tools, but operators need trustworthy proof. Clawdlinux applies governance around agent workloads. Today I will show real model evidence, configuration controls, and tamper detection."

### 0:35-0:55. Start the live view

**Operator action:** Run `python3 scripts/demo-visualizer.py --present --tamper-audit`. Open `http://127.0.0.1:8765` during the countdown. Make it full-screen.

**Screen state expected:** The branded control room loads. Panels show pending states before evidence arrives.

**Exact words to say:**

"I started one command. This dashboard is a passive view of the script's stdout. It does not create evidence. The terminal remains the source stream."

### 0:55-2:10. Real workload and model evidence

**Operator action:** Point to the LIVE panel as the AgentWorkload completes. Show routing, tokens, cost, and Claude reachability.

**Screen state expected:** LIVE shows Completed, nonzero tokens, estimated cost, OpenAI routing, and the separate Anthropic check.

**Exact words to say:**

"This is a real AgentWorkload applied through the repo-local agentctl. The workload routes through in-cluster LiteLLM to the OpenAI alias clawdlinux-openai. The completion, token counts, and estimated cost are real. This smaller Claude event is a separate reachability call through LiteLLM. The system is provider-independent, but this hero workload uses OpenAI today."

### 2:10-2:50. Configuration evidence and limits

**Operator action:** Point to CONFIG ONLY. Read both control states without expanding their claims.

**Screen state expected:** gVisor says dry-run injected with no pod scheduled. NetworkPolicy says object present with no packet test.

**Exact words to say:**

"These are configuration proofs, not runtime enforcement proofs. A server-side dry-run shows the webhook injecting runtimeClassName gvisor. No pod was scheduled. The NetworkPolicy object exists. We did not test packet enforcement, which also depends on the cluster CNI."

### 2:50-4:05. Prior-run audit and tamper rejection

**Operator action:** Point to PRIOR RUN. Show the verified receipt, then the tamper rejection.

**Screen state expected:** The prior-run HMAC fixture passes. A temporary modified copy is rejected. The event tail shows both results.

**Exact words to say:**

"This receipt is a prior-run HMAC audit fixture. The current workload did not generate it. First, the verifier accepts the unchanged artifact. Then the demo modifies a temporary copy. Verification rejects that copy, proving recorded evidence cannot be changed without detection."

### 4:05-4:40. Product and current proof

**Operator action:** Hold on the 3 evidence panels. Do not navigate away.

**Screen state expected:** LIVE, CONFIG ONLY, and PRIOR RUN remain visible with the 5-line event tail.

**Exact words to say:**

"The product is the in-cluster governance and attestation layer around agent runtimes. Today's proof shows real provider routing, measured usage, admission mutation, policy objects, and offline tamper detection. Full runtime enforcement and same-run signed attestation remain the next proof steps."

### 4:40-5:00. Closing ask

**Operator action:** Keep the completed control room visible. Stop moving the pointer.

**Screen state expected:** The final evidence remains readable. No slide replaces it during the screening.

**Exact words to say:**

"For Thursday, I want to show this same honest proof with the final enforcement path connected. I am asking for a showcase slot to demonstrate agent actions that are governed, measured, and independently verifiable."

## Exact 90-second fallback rule

Start a 90-second timer when the live command begins.

If the workload is not Completed by 90 seconds, say this exact sentence:

"The live workload has not completed within my 90-second limit, so I am switching visibly to a recorded successful rehearsal."

Press `Ctrl+C` in the live terminal. Then run:

```bash
python3 scripts/demo-visualizer.py --replay demos/demo-20260713.log
```

Open or refresh `http://127.0.0.1:8765` during its countdown. Confirm the screen says RECORDED REHEARSAL.

Say: "This is a recorded successful rehearsal, not a live run. I will continue from the same panel."

Resume at the panel where the live run stopped. Never call the replay live.

If the browser fails, return to the terminal. Say: "The terminal is the source stream, so I will continue there."

Never swap to replay silently.

## Questions likely in screening

### Why is there a Claude event if the hero is OpenAI?

"The hero AgentWorkload uses the OpenAI alias today. Claude receives a separate small reachability call through the same LiteLLM path."

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

## Thursday two bookend slides

These slides are optional for Thursday. Do not use them in this screening.

### Slide 1. Problem

**One sentence:** AI agent actions need evidence that remains trustworthy after execution.

- Agents cross model, tool, and runtime boundaries.
- Existing controls prove separate layers, not the full action trail.
- Clawdlinux joins governance with tamper-evident receipts.

### Slide 2. Proof and ask

- Proof: real routing, tokens, cost, control configuration, and tamper rejection.
- Ask: include the live enforcement path in the Thursday showcase.
- GitHub: `https://github.com/Clawdlinux/agentic-operator-core`

Protect the closing slide. Under time pressure, cut the opener slide first.

## Do not say

- Do not say the hero workload uses Claude.
- Do not say the Claude check is the AgentWorkload completion.
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