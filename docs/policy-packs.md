# Policy packs

Policy packs are versioned Rego rule sets. The operator evaluates them with
the OPA Go library embedded in `pkg/rules/engine`. Packs ship inside the
operator binary (`pkg/rules/packs`). No network fetch, so they work air-gapped.

Packs are engineering controls. They are NOT legal advice. They are NOT a
certification. Enabling a pack does not make a workload compliant with any
law. Each pack covers a few checks only.

## Enable a pack

```yaml
apiVersion: agentic.clawdlinux.org/v1alpha1
kind: AgentWorkload
spec:
  opaPolicy: strict
  declaredIntent:
    purpose: "support replies to customers"
    decisionType: assisted
    allowedDataClasses: [email]
    allowedDestinations: [mcp.internal]
  policyPacks:
    - gdpr-eu@v0.1.0
```

Rules for `spec.policyPacks`:
- Each entry is `name@vX.Y.Z`. The CRD rejects other shapes.
- The version must match a shipped pack exactly. The validating webhook
  rejects unknown packs. If the webhook is off, the controller fails the
  workload closed: phase `Failed`, condition `PolicyPackInvalid`, no MCP call.
- Packs run on the legacy direct-action path only. A workload with
  `spec.orchestration` and `spec.policyPacks` fails closed the same way.

## Shipped packs

| Pack | Triggers on | Deny | Require approval |
|---|---|---|---|
| [`dpdp-in@v0.1.0`](../pkg/rules/packs/dpdp-in@v0.1.0/README.md) | `aadhaar`, `pan` | empty `purpose`, empty `decisionType`, class not in `allowedDataClasses` | `decisionType: automated` |
| [`gdpr-eu@v0.1.0`](../pkg/rules/packs/gdpr-eu@v0.1.0/README.md) | `email`, `phone`, `iban`, `card` | empty `purpose`, class not in `allowedDataClasses` | `decisionType: automated` (Art. 22 style) |

## How packs combine with other layers

Order: [invariants](architecture/invariants.md), then packs, then the
threshold evaluator. The strictest result wins.

- A pack can only add deny or require approval. It cannot allow anything.
- A pack cannot lift an invariant deny.
- A pack approval finding never auto-allows. The workload goes to
  `PendingApproval` with condition `ApprovalRequired`, in strict or permissive
  mode.
- A pack load or eval error counts as deny.
- Agent-claimed confidence cannot loosen a pack result.

Findings appear in condition messages as `pack/RULE-ID: reason`, for example
`gdpr-eu@v0.1.0/GDPR-EU-01: personal data observed without a declared purpose`.

## Input document

The engine passes this input to every pack. Built by `engine.Doc` from
[policy input](policy-input.md).

```json
{
  "declared": {"purpose": "", "decisionType": "", "allowedDataClasses": [], "allowedDestinations": []},
  "observed": {"destination": "", "dataClasses": [], "tool": "", "callerIdentity": "",
               "priorActionCount": 0, "priorDeniedCount": 0},
  "claimed":  {"intent": "", "confidence": 0.9, "clusterHealth": 80}
}
```

`claimed.confidence` and `claimed.clusterHealth` are omitted when absent.
Treat everything under `claimed` as untrusted.

## Write a pack

1. Create `pkg/rules/packs/<name>@v<X.Y.Z>/`. Never edit a released version.
   Copy it to a new version directory instead.
2. Use `package clawdlinux.decision`. The engine rejects any other package.
3. Emit findings as objects with string `id` and `reason`:

```rego
package clawdlinux.decision

deny contains {"id": "ACME-01", "reason": "card data needs a purpose"} if {
	"card" in input.observed.dataClasses
	object.get(input, ["declared", "purpose"], "") == ""
}

require_approval contains {"id": "ACME-02", "reason": "automated card decision"} if {
	"card" in input.observed.dataClasses
	input.declared.decisionType == "automated"
}
```

4. Only `deny` and `require_approval` are read. Any other rule is ignored.
5. Add `*_test.rego` files next to the rules. `go test ./pkg/rules/packs/`
   runs them like `opa test`.
6. Add a `README.md` with the rule table and the not-legal-advice notice.
7. Add the name to the `NamePattern` in `pkg/rules/packs` and the CRD
   `items:Pattern` marker on `PolicyPacks`, then run `make manifests`.

## Not evaluated

The Rego samples in `config/policies` and `pkg/rules/threshold/policies.rego`
are not evaluated by the engine. Only shipped packs listed in
`spec.policyPacks` run.
