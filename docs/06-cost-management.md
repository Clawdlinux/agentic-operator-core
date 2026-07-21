# Cost Management

Estimate and expose AI workload usage through the `CostReporter` interface.

The open-source operator defaults to `NoOpCostReporter`. Cost is not recorded or
enforced unless another reporter is configured. `MemoryCostReporter` is intended
for demos and local evaluation. Its data is lost when the process restarts.

## Token Tracking

With a configured reporter, successful model calls can record prompt and completion tokens:

```yaml
Status:
  Conditions:
    - Type: ModelRoutingSucceeded
      Message: "routed to cloudflare-workers-ai/llama-2-7b (input:100 tokens, output:250 tokens)"
```

These values support estimated cost. They are not a billing ledger.

## Per-Provider Costs

Costs vary by provider and model:

```
Cloudflare Workers AI:
  - llama-2-7b-chat: $0.0002 / 1K tokens
  - mistral-7b: $0.0003 / 1K tokens

OpenAI:
  - gpt-4: $0.03 / 1K prompt, $0.06 / 1K completion
  - gpt-4-turbo: $0.01 / 1K prompt, $0.03 / 1K completion
```

## Routing

Model selection optimizes for cost:

```yaml
modelStrategy: cost-aware
```

Routing and cost reporting are separate paths. Do not treat the in-memory estimate
as a production provider invoice.

## Budget Enforcement

Tenant CRDs expose token quotas. Production enforcement requires the relevant
controller and reporter integration.

```yaml
quotas:
  maxMonthlyTokens: 10000000
```

Configured budget reporters may reject workloads after a limit is reached:
```
Error: Monthly token budget exceeded for tenant
```

## Cost Reporting

Query usage metrics:

```bash
# Total workloads
kubectl get agentworkload -A --sort-by=.status.conditions[0].message | grep routed

# Cost breakdown per provider
kubectl get agentworkload -A -o json | \
  jq -r '.items[] | .status.conditions[0].message' | \
  grep -oP 'routed to \K[^ ]+' | sort | uniq -c
```

## Optimization Tips

1. Configure a durable `CostReporter` before relying on chargeback.
2. Validate model pricing against the provider contract.
3. Set workload and tenant limits from measured usage.
4. Alert on token, request, and estimated-cost changes.
5. Reconcile estimates against provider invoices.

See `Monitoring` for detailed cost dashboards.
