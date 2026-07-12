# Pitch Script: Co-founder Deck + Demo

Companion to the current co-founder deck and [DEMO-PLAN.md](DEMO-PLAN.md). Total runtime: ~10 min deck, ~10 min demo, rest conversation. The deck earns attention, the demo earns belief, the conversation earns the signal. Do not skip the conversation to show more product.

## Part 1: Deck script, slide by slide

Times are targets. If she interrupts with questions, good, let her. The deck is a spine, not a cage.

**Slide 1, Title (30s).**
"Clawdlinux is building regulated controls for AI agents on Kubernetes. The target contract is offline-verifiable attestation with packet-enforced egress controls. Today's booth proof is narrower, and I will label every boundary. I am not building another agent framework. I am building the control plane that lets a bank review one."

**Slide 2, Problem (90s).**
"Agents work now. What does not work is getting them past a security review. These are not my numbers." Walk the rings left to right. Land on: "Gartner says 40 percent of these projects die, and names inadequate risk controls as a reason. Only 11 percent of production agents pass an independent security bar. The demand curve and the controls curve have a gap. That gap is the company."

**Slide 3, Why now (90s).**
"Four things happened at once. Strong open runtimes became available, so we no longer need to rebuild that layer. Incidents went from hypothetical to a bank self-reporting to its regulator. Fed guidance explicitly excludes agentic AI, so auditors are improvising. The buyers who need verifiable evidence most are also the ones that often cannot use a hosted control plane. We start with self-hosted and air-gapped deployments."

**Slide 4, What we built (90s).**
"Our target product is an in-cluster governance and attestation plane. The booth proves one OpenAI-routed model call through LiteLLM. It shows genuine tokens, nonzero cost evidence, and a separate Anthropic reachability check. It also shows webhook mutation simulation, NetworkPolicy object presence, and prior-run HMAC audit verification. The current path does not execute OPA. It does not prove packet enforcement, a scheduled gVisor workload, or current-run audit generation."

**Slide 5, How a run works (60s).**
Walk the six target steps quickly. Say: "This slide describes the target product contract. Policy allow or deny happens before execution in that contract. The current `--present` path does not execute OPA. It also does not produce current-run attestation."

**Slide 6, Use case (90s).**
"Target customer: a fund's platform team wants agents analyzing filings and market data. Infosec banned agent SaaS, so the project is stuck. In the target deployment, they install one bundle inside their perimeter. They point LiteLLM at an approved model and label existing agent pods. Platform ships the workload. Security reviews the controls. Compliance receives the product attestation package."

**Slide 7, What exists (60s).**
"Everything on this slide is in a public repo you can read tonight. 180 commits since February. The ACP numbers are measured, checked-in benchmarks, not projections: 64 to 97 percent context reduction against raw MCP, one round trip instead of up to 21."

**Slide 8, Where we are (60s). Do not rush this one.**
"Honest state: built but not validated. Pre-revenue, zero customers, and I have a kill criterion: ten real conversations with named platform and security engineers by July 15, or I stop building features until I have them. I am telling you this because if you join, your first job is making sure we never ship into a vacuum again."

**Slide 9, Roadmap (45s).**
"The roadmap has gates, not wishes. SPIFFE federation, SOC 2, managed offering: all of it waits for a customer to pull it. The only committed work is validation and the July 22 booth."

**Slide 10, The ask (90s).**
"Why you. This product is a trust claim, and trust claims need a security person whose name carries weight. Three things you would own: the threat model, attack our own chain before a red team does; compliance mapping, what will SEC, FINRA, RBI examiners actually accept; and security-side go-to-market, the CISO conversations I cannot credibly have alone. Concrete next step if you are in: we pick one design-partner profile together and you pressure-test the demo like a hostile auditor. Want to see it now?"

Transition straight into the demo. No gap.

## Part 2: Current booth proof and target product contract

Prepare once with both provider keys exported or stored in the ignored repo-root `.env` file:

```bash
scripts/demo-booth.sh --prepare
```

Set `DEMO_ENV_FILE=/path/to/file` to use another credential file.
Exported variables override file values.
The script parses only the 2 provider key names.
It never sources the file or prints values.

Preparation creates kind cluster `clawdlinux-demo`. It installs pinned cert-manager.
It creates one runtime Secret without printing values. Helm receives only the Secret name.

Run the 5-7 minute path:

```bash
scripts/demo-booth.sh --present
```

### CURRENT BOOTH PROOF

Say each proof label aloud. Do not blend target behavior into the current demo.

**REAL LLM CALL AND COST.** `examples/research-agent.yaml` uses one incident investigator.
Every task category maps to `clawdlinux-openai`. LiteLLM routes that alias to OpenAI.
The status condition shows genuine token counts. The annotation and metric show nonzero cost.

**REAL, separate Anthropic check.** A small request uses `clawdlinux-anthropic` through LiteLLM.
It proves provider reachability. Do not claim the AgentWorkload used Anthropic.

**WEBHOOK MUTATION SIMULATION.** A server-side dry-run shows webhook mutation.
It injects `runtimeClassName=gvisor`. No gVisor pod is scheduled on kind/macOS.

**NETWORKPOLICY OBJECT PRESENCE ONLY.** The script shows the generated NetworkPolicy object.
Say: "Packet enforcement requires an enforcing CNI." This iteration installs no Cilium.

**PRIOR-RUN HMAC AUDIT FIXTURE.** The checked-in JSONL fixture passes HMAC verification offline.
The current workload did not generate it. Do not call it a current-run artifact or asymmetric signature.

Add `--tamper-audit` to alter a temporary copy and show verification failure.
Use `scripts/record-demo.sh` to record the present path.

**OPA IS LEGACY OR TARGET ONLY.** The legacy default flow retains its OPA allow/deny development path.
The target product contract also includes policy evaluation. `--present` executes neither path.

**What this demo does not prove.** It does not prove packet enforcement on kind.
It does not prove a scheduled gVisor workload. It does not prove current-run attestation.
It does not prove every runtime pod has a sandbox label. It does not prove an OPA decision or policy gate.

### TARGET PRODUCT CONTRACT

The target product emits offline-verifiable, tamper-evident attestation for each governed run.
The target product applies packet-enforced egress controls through a supported enforcing CNI.
These are product acceptance criteria. They are not outcomes from the current booth path.

## Part 3: Turning the demo into signal and customers

The demo is not the product. The demo is bait for a specific conversation. The high-value signal is not "cool demo", it is evidence someone would deploy or pay.

**Signal ladder, weakest to strongest.** Compliment < asks technical questions < describes their own environment unprompted < names a specific blocked project < agrees to a follow-up with a named colleague < asks for the install bundle < agrees to a scoped pilot with dates. Everything below "describes their own environment" counts toward nothing. Your Jul 15 kill criterion counts only conversations where they talk about THEIR stack, not yours.

**The three questions to ask everyone who sees the demo.** Ask, then shut up:
1. "Do you have an agent project that is stuck right now? What is it stuck on?" (Discovers whether the pain is real and whether it is governance-shaped.)
2. "Who in your org would have to say yes to something like this, and what would they need to see?" (Maps the buying committee and gives you the artifact list for the pilot.)
3. "If I gave you the air-gapped bundle Monday, what would stop you from installing it?" (Surfaces the real objection: procurement, priority, missing feature, or no actual pain.)

**The one CTA.** Free design-partner pilot for regulated enterprises, already on the website. Scope it tight when you offer it: "30 days, one use case, your cluster, we help install, you give us a weekly 30-minute call and an honest verdict." A tight scope is easier to say yes to and produces a reference or a lesson either way.

**Disqualify fast.** No Kubernetes, no regulated pressure, or no stuck project: be polite, take nothing, move on. A pipeline of unqualified enthusiasm is how the last two engineering-over-validation cycles happened.

**Follow-up discipline.** Same-day note with exactly three things: the repo link, the one thing they said they cared about, and the single next step you proposed. No decks attached unless they asked.

**For the Shrishti call specifically.** The customer-signal framing doubles as the co-founder test. If she starts asking question 2 and 3 style questions at YOU, unprompted, that is the strongest positive signal you can get from a security co-founder candidate: she is already selling it in her head. If she only critiques the crypto, she is an advisor, not a co-founder. Both outcomes are useful; know which one you got before the call ends.
