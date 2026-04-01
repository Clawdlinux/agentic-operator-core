# Research Swarm Validation Runbook

Use this runbook to prove the 10-minute demo works from a clean local environment.

## Preconditions

- Docker + Docker Compose installed
- `make`, `curl`, `jq` installed
- Azure AI Foundry vars configured in `.env`:
   - `AZURE_OPENAI_ENDPOINT`
   - `AZURE_OPENAI_API_KEY`
   - `AZURE_OPENAI_API_VERSION`

## One-command validation

```bash
make validate-e2e
```

This command:

1. Tears down any old local stack state.
2. Rebuilds demo images.
3. Starts all services.
4. Waits for health endpoints.
5. Executes one end-to-end orchestration request.
6. Verifies:
   - three completed stages
   - non-empty final output
   - positive total cost
   - cost endpoint readability

## Expected output

- `Validation PASS`
- `trace_id=<uuid>`
- `total_cost_usd=<positive value>`

## Troubleshooting

- If health checks fail: run `make logs` and inspect service boot errors.
- If orchestration fails: confirm `.env` has valid Azure AI Foundry values.
- If cost check is empty: ensure LiteLLM proxy is healthy at `http://localhost:8000/health`.
