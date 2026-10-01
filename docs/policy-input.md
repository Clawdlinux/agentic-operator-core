# Policy input

Every action decision reads inputs from three sources. Each source has a
different trust level. Source of truth for the design:
[decision architecture](architecture/decision-architecture.md).

| Class | Who sets it | Can it loosen a decision? |
|---|---|---|
| Declared | A human, in the manifest, reviewed in a PR | Yes, it defines what is allowed |
| Observed | The platform, measured at decision time | Yes, but only within Declared |
| Agent-claimed | The agent or its MCP server | No. It can only tighten |

Code: `pkg/decision/input` (types), `pkg/decision/observe` (builder),
`pkg/dataclass` (detectors), `pkg/invariants` (always-on checks),
`pkg/rules/engine` (policy packs), `pkg/decision.Decide` (strictest wins).

## Declared

Set in `spec.declaredIntent` on an `AgentWorkload`. All fields optional.

| Field | Meaning |
|---|---|
| `purpose` | Short human statement of what the workload does. Policy packs require it when they see personal data. |
| `decisionType` | Kind of decision the agent makes, e.g. `refund-triage`. Packs treat `automated` as needing human approval when personal data is present. |
| `allowedDataClasses` | Data classes the workload may send. Known: `email`, `phone`, `aadhaar`, `pan`, `iban`, `card`. Declaring `credential` has no effect: INV-01 always denies it. |
| `allowedDestinations` | Hosts the workload may call. Exact host, or `*.example.com` for any subdomain. The apex does not match the wildcard. |

Rules:
- If `declaredIntent` is absent, declared-intent checks (INV-02 to INV-04)
  do not run. INV-01 (credentials) still runs.
- If `declaredIntent` is present, an empty allow list allows nothing.
- Matching is case-insensitive.

Sample: [`config/samples/agentworkload_declared_intent.yaml`](../config/samples/agentworkload_declared_intent.yaml).

## Observed

Built by `observe.Observe` from values the controller already holds.

| Field | Source today |
|---|---|
| `destination` | Host part of `spec.mcpServerEndpoint`. The proposed action is sent there via `execute_action`. |
| `dataClasses` | `pkg/dataclass` run over the `propose_action` reply (action, description, params). Class names only. Matched content is never stored or logged. |
| `tool` | The proposed action name. |
| `callerIdentity` | Empty. `AgentWorkload` has no caller or actor field today. |
| `priorActionCount` | `len(status.executedActions) + len(status.proposedActions)`. |
| `priorDeniedCount` | Proposed actions with `approved: false`. |

Detectors are deterministic. Aadhaar requires a valid Verhoeff checksum. Card
requires Luhn. IBAN requires mod-97. Phone is pattern based (E.164 with `+`,
10 digit Indian mobile, US `(415) 555-0100`) and will have false positives.
Credential is pattern based: AWS access key IDs, PEM private key headers,
JWT-shaped tokens, `Bearer` tokens of 20+ characters, and GitHub `ghp_` and
`github_pat_` tokens.
Scanning is bounded: depth 8, 64 KiB per string, 1 MiB and 10000 nodes per
value. Map keys are not scanned.

### Not observed yet

Be clear about the gaps:
- Caller identity. No field carries it. A future change can read the
  requesting user from an admission webhook or a signed annotation.
- Real egress. Destination is the configured MCP endpoint. Hosts the MCP
  server itself calls are not seen by this check.
- Tokens and cost per action.
- Data sent before the decision. The objective already goes to the MCP server
  in `propose_action`, before any check runs.
- Cluster health. The platform does not measure it. See below.
- Orchestrated runtimes (`spec.orchestration`). Invariants and packs run only
  on the legacy direct-action path. Packs set on an orchestrated workload
  fail closed with condition `PolicyPackInvalid`.

## Agent-claimed

| Field | Source today |
|---|---|
| `confidence` | `confidence` in the `propose_action` reply. |
| `clusterHealth` | `cluster_health` in the MCP `get_status` reply. Nil when absent. The threshold evaluator then uses a default of 75. |
| `intent` | `description` in the `propose_action` reply. |

These values feed the threshold evaluator in `pkg/rules/threshold`. They are
logged as agent-claimed. They may only tighten an outcome. A high claimed
confidence cannot turn an invariant or pack finding into an allow.

## Outcome

`decision.Decide` merges the layers. The strictest result wins.

| Result | strict | permissive or unset |
|---|---|---|
| Any invariant finding ([list](architecture/invariants.md)) | `PolicyDenied` | `PendingApproval` |
| Any pack deny or pack error | `PolicyDenied` | `PendingApproval` |
| Threshold deny | `PolicyDenied` | `PendingApproval` |
| Pack require approval, nothing denies | `PendingApproval` | `PendingApproval` |
| Nothing objects | action executes | action executes |

Under strict, reasons with rule IDs (for example `INV-02: ...` or
`gdpr-eu@v0.1.0/GDPR-EU-01: ...`) are appended to the `PolicyDenied`
condition message. A pack approval sets condition `ApprovalRequired`. In
permissive mode a deny sets no condition, as before. See
[policy packs](policy-packs.md).
