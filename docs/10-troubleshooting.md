# Troubleshooting

Common issues and solutions.

## Tenant Provisioning Fails

**Symptom:** Tenant status stuck in "Provisioning"

**Diagnosis:**
```bash
kubectl describe tenant <name>
kubectl logs -n agentic-system -l app=agentic-operator
```

**Solutions:**

1. **Namespace creation permission**
   ```bash
   kubectl auth can-i create namespaces --as=system:serviceaccount:agentic-system:agentic-operator
   ```

2. **Secret not found in agentic-system**
   ```bash
   kubectl get secrets -n agentic-system | grep cloudflare
   ```
   → Create missing secrets

3. **RBAC role creation failed**
   ```bash
   kubectl get roles -n agentic-customer-*
   ```
   → Check operator logs for permission errors

## Workload Never Completes

**Symptom:** AgentWorkload stays in "Pending"

**Diagnosis:**
```bash
kubectl describe agentworkload <name> -n <namespace>
```

**Solutions:**

1. **Provider secret missing**
   ```bash
   kubectl get secrets -n <namespace> | grep api-token
   ```

2. **License expired**
   - Check operator logs for license validation errors
   - Update license key

3. **Quality threshold too high**
   - Lower `autoApproveThreshold`
   - Check evaluation logs

## High Costs

**Symptom:** Token usage exceeds expectations

**Solutions:**

1. **Switch to cost-aware routing**
   ```yaml
   modelStrategy: cost-aware
   ```

2. **Lower quality threshold**
   ```yaml
   autoApproveThreshold: 0.7  # From 0.95
   ```

3. **Monitor per-workload costs**
   ```bash
   kubectl get agentworkload -A -o json | \
     jq '.items[] | .status.conditions[0].message'
   ```

## Quota Exceeded

**Symptom:** "Monthly token budget exceeded"

**Solution:**
```bash
# Increase quota
kubectl patch tenant <name> --type merge -p \
  '{"spec":{"quotas":{"maxMonthlyTokens":50000000}}}'
```

## Common Errors

| Error | Cause | Fix |
|-------|-------|-----|
| Secret not found | Secret in wrong namespace | Copy to tenant namespace |
| Authorization denied | Insufficient RBAC | Check service account roles |
| Provider error | API key invalid | Verify and rotate secrets |
| OPA policy violation | Policy too strict | Review or relax policies |

See API Reference for detailed error codes.
