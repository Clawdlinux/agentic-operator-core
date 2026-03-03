# Cost Management

Track, control, and optimize AI workload costs.

## Token Tracking

Every workload execution is metered:

```yaml
Status:
  Conditions:
    - Type: ModelRoutingSucceeded
      Message: "routed to cloudflare-workers-ai/llama-2-7b (input:100 tokens, output:250 tokens)"
```

Input + output tokens are captured for billing.

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

## Cost-Aware Routing

Model selection optimizes for cost:

```yaml
modelStrategy: cost-aware
```

Operator selects cheapest provider that meets quality thresholds.

## Budget Enforcement

Tenants have monthly token quotas:

```yaml
quotas:
  maxMonthlyTokens: 10000000
```

Once reached, workloads are rejected:
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

1. **Use cost-aware routing** - Balances cost and quality
2. **Right-size quotas** - Set realistic monthly limits
3. **Monitor trends** - Watch for cost spikes
4. **Batch requests** - Fewer, larger requests vs many small ones
5. **Select efficient models** - Smaller models for simple tasks

See `Monitoring` for detailed cost dashboards.
