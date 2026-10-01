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
`pkg/dataclass` (detectors), `pkg/decision.Combine` (tighten-only merge).

## Declared

Set in `spec.declaredIntent` on an `AgentWorkload`. All fields optional.

| Field | Meaning |
|---|---|
| `purpose` | Short human statement of what the workload does. Recorded only. |
| `decisionType` | Kind of decision the agent makes, e.g. `refund-triage`. Recorded only. |
| `allowedDataClasses` | Data classes the workload may send. Known: `email`, `phone`, `aadhaar`, `pan`, `iban`, `card`. |
| `allowedDestinations` | Hosts the workload may call. Exact host, or `*.example.com` for any subdomain. The apex does not match the wildcard. |

Rules:
- If `declaredIntent` is absent, nothing changes. The legacy path behaves as before.
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
- Orchestrated runtimes (`spec.orchestration`). This check runs only on the
  legacy direct-action path.

## Agent-claimed

| Field | Source today |
|---|---|
| `confidence` | `confidence` in the `propose_action` reply. |
| `clusterHealth` | `cluster_health` in the MCP `get_status` reply. Nil when absent. The threshold evaluator then uses a default of 75. |
| `intent` | `description` in the `propose_action` reply. |

These values feed the existing threshold evaluator in `pkg/opa`. They are
logged as agent-claimed. They may only tighten an outcome.
`decision.Combine(rulesDenied, thresholdAllowed)` returns
`!rulesDenied && thresholdAllowed`. A high claimed confidence cannot turn a
declared-intent violation into an allow.

## Outcome

When `declaredIntent` is set and any violation is found:
- strict policy: phase `PolicyDenied`, violation reasons appended to the
  `PolicyDenied` condition message.
- permissive or unset policy: phase `PendingApproval`.
