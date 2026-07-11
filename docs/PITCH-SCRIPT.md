# Pitch Script: Co-founder Deck + Demo

Companion to the current co-founder deck and [DEMO-PLAN.md](DEMO-PLAN.md). Total runtime: ~10 min deck, ~10 min demo, rest conversation. The deck earns attention, the demo earns belief, the conversation earns the signal. Do not skip the conversation to show more product.

## Part 1: Deck script, slide by slide

Times are targets. If she interrupts with questions, good, let her. The deck is a spine, not a cage.

**Slide 1, Title (30s).**
"Clawdlinux provides regulated controls for AI agents on Kubernetes. One sentence version: every agent run leaves a signed, offline-verifiable record, and runs inside a network seal it cannot escape. I am not building another agent framework. I am building the thing that lets a bank say yes to one."

**Slide 2, Problem (90s).**
"Agents work now. What does not work is getting them past a security review. These are not my numbers." Walk the rings left to right. Land on: "Gartner says 40 percent of these projects die, and names inadequate risk controls as a reason. Only 11 percent of production agents pass an independent security bar. The demand curve and the controls curve have a gap. That gap is the company."

**Slide 3, Why now (90s).**
"Four things happened at once. Strong open runtimes became available, so we no longer need to rebuild that layer. Incidents went from hypothetical to a bank self-reporting to its regulator. Fed guidance explicitly excludes agentic AI, so auditors are improvising. The buyers who need verifiable evidence most are also the ones that often cannot use a hosted control plane. We start with self-hosted and air-gapped deployments."

**Slide 4, What we built (90s).**
"Our wedge combines two controls in one self-hosted deployment. First, attestation: every LLM call, tool call, and approval goes into a hash chain, HMAC-signed, and a small CLI recomputes the whole chain offline. The auditor does not have to trust me, or you, or the customer. Second, the egress seal: the agent reaches only the endpoints its manifest declares, enforced by Cilium at the network layer. Not a system prompt asking the model to behave. gVisor and OPA wrap around those." Pause. "Key design decision: same contract whether the workload is our CRD, someone's existing pods, or a third-party runtime. We wrap, we do not replace."

**Slide 5, How a run works (60s).**
Walk the six steps quickly. The line that matters: "Notice policy is decided before anything runs, and evidence is produced whether or not anyone asks for it. Compliance is a byproduct of running, not a project after running."

**Slide 6, Use case (90s).**
"Concrete customer: a fund's platform team wants agents analyzing filings and market data. Infosec banned agent SaaS, so the project is stuck. With us: they helm install one bundle inside their perimeter, point LiteLLM at whatever model they already approved, and label their existing agent pods. Three people get unblocked: platform lead ships, security reviews a YAML diff instead of a prompt, compliance hands the auditor a snapshot and a binary instead of a week of log archaeology."

**Slide 7, What exists (60s).**
"Everything on this slide is in a public repo you can read tonight. 180 commits since February. The ACP numbers are measured, checked-in benchmarks, not projections: 64 to 97 percent context reduction against raw MCP, one round trip instead of up to 21."

**Slide 8, Where we are (60s). Do not rush this one.**
"Honest state: built but not validated. Pre-revenue, zero customers, and I have a kill criterion: ten real conversations with named platform and security engineers by July 15, or I stop building features until I have them. I am telling you this because if you join, your first job is making sure we never ship into a vacuum again."

**Slide 9, Roadmap (45s).**
"The roadmap has gates, not wishes. SPIFFE federation, SOC 2, managed offering: all of it waits for a customer to pull it. The only committed work is validation and the July 22 booth."

**Slide 10, The ask (90s).**
"Why you. This product is a trust claim, and trust claims need a security person whose name carries weight. Three things you would own: the threat model, attack our own chain before a red team does; compliance mapping, what will SEC, FINRA, RBI examiners actually accept; and security-side go-to-market, the CISO conversations I cannot credibly have alone. Concrete next step if you are in: we pick one design-partner profile together and you pressure-test the demo like a hostile auditor. Want to see it now?"

Transition straight into the demo. No gap.

## Part 2: What the demo actually is, technically

`scripts/demo-booth.sh` is a gate script: it provisions and then proves five controls, printing PASS/FAIL evidence for each. Know what each phase does so you can narrate it instead of reading it.

**Phase 0, setup.** Checks kubectl, helm, kind (or reuses k3s). Creates kind cluster `ninevigil-demo`. The platform profile installs the operator, Argo, and shared services via `tests/harness/setup.sh`. `--profile lean` installs the CRD, operator, NetworkPolicy, and gVisor sandbox while omitting Argo and shared services. The lean profile starts faster and does not depend on Argo scheduling; use it when the laptop or wifi is the risk.

**Phase 1, OPA allow/deny.** Applies `opa-allow-demo` (objective: "analyze quarterly revenue data") and waits for it to reach a running phase. Then applies `opa-deny-demo` (objective: "delete all production volumes") and waits for phase `PolicyDenied`, then prints the deny reason from the CRD status condition. What this proves: policy is evaluated by the operator against the manifest before execution, and the decision plus the reason is recorded on the Kubernetes object itself. The model is not in the loop.

**Phase 2, cost.** Curls the operator's metrics endpoint from inside the pod and greps for `ninevigil_agent_cost_dollars`. What this proves: per-workload cost is a first-class metric, not a spreadsheet.

**Phase 3, gVisor.** Confirms the `gvisor` RuntimeClass exists, then does a server-side dry-run of a pod labeled `agentic.clawdlinux.org/runtime-sandbox: gvisor` and shows that the mutating webhook injected `runtimeClassName: gvisor`. What this proves: isolation is applied by admission control based on a label. No agent code changes.

**Phase 4, NetworkPolicy.** Confirms the egress policy objects are installed in the namespace. What this proves: the seal exists as a Kubernetes-native object security can review. (On kind, Cilium FQDN enforcement is not fully live; do not claim packet-level blocking in this environment, claim policy generation and show the object.)

**Phase 5, swarm (`--with-swarm` only).** Applies `demo-competitive-swarm`: three agents (competitor-scraper, llm-synthesizer, report-generator) compiled into an Argo Workflow DAG. What this proves: the same controls wrap a multi-agent pipeline, and orchestration is delegated to Argo rather than reinvented.

**The audit tamper closer uses the signed fallback artifact.** Follow [`_staging/booth/README.md`](../_staging/booth/README.md): run `audit-verify --source jsonl --path _staging/booth/attestation-fallback.jsonl --key <demo-kid=base64-key>` for PASS, use the documented `sed` command to create `/tmp/tampered.jsonl`, then verify that file for FAIL. Rehearse this end to end at least once before the call. If you cannot get a clean PASS/FAIL cycle working, cut it from the live demo and show the `pkg/audit` design instead; a fumbled trust demo is worse than none.

**Flags cheat sheet.** `--profile platform|lean` selects the deployment profile, `--with-swarm` adds the Argo swarm scenario, `--record` captures the terminal with script(1), and `--cleanup` deletes the cluster. Evidence lands in `tests/harness/evidence/booth-<timestamp>/`.

**What the demo does NOT show. Say so if asked.** No real LLM traffic unless an endpoint is configured; the allow workload exercises lifecycle, not inference quality. No packet-level egress blocking on kind. No SPIFFE, no multi-cluster, no SOC 2. Saying "that part is roadmap" out loud is what makes the rest believable.

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
