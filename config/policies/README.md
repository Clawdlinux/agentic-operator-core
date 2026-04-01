# OPA Policy Library

Reusable policy samples for common AgentWorkload guardrails.

## What is included

- `samples/budget-cap.rego` enforces monthly or per-run budget ceilings.
- `samples/egress-allowlist.rego` blocks outbound domains outside an allow-list.
- `samples/model-allowlist.rego` restricts providers and models to approved sets.

## Package into a ConfigMap

```bash
kubectl apply -k config/policies
```

This creates `ConfigMap/agentic-opa-policy-library` in `agentic-system`.

## Validate a policy locally

```bash
opa eval -d config/policies/samples/budget-cap.rego \
  -i input.json 'data.agentic.policies.budget.decision'
```

## Expected policy input shape

```json
{
  "policy_name": "budget-cap",
  "budget_cap_usd": 250,
  "spend_month_to_date_usd": 220,
  "estimated_cost_usd": 12,
  "requested_egress_domains": ["api.openai.com"],
  "allowed_egress_domains": ["api.openai.com", "github.com"],
  "provider": "openai",
  "model": "gpt-4o",
  "allowed_providers": ["openai", "anthropic"],
  "allowed_models": ["gpt-4o", "gpt-4o-mini", "claude-3-5-sonnet"]
}
```
