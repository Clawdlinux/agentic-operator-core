# Research Swarm Demo

A self-contained Docker Compose demo showing a 3-agent research pipeline with **cost tracking, agent isolation, and multi-agent orchestration**.

## What This Demo Shows

This demo demonstrates core operator capab ilities:

| Feature | How It Works |
|---------|-------------|
| **Cost Tracking** | Each agent has a virtual API key; LiteLLM tracks spend per agent and total per run |
| **Agent Isolation** | Researcher, Writer, Editor run in separate containers with distinct personas (role, tone) |
| **Multi-Agent Swarm** | Pipeline orchestrator chains agents sequentially: research outline → draft prose → final polished article |
| **Per-Agent Persona** | Each agent has distinct system prompt guidance (tone: neutral_academic, engaging_professional, authoritative_precise) |
| **Artifact Tracking** | MinIO stores intermediate outputs (outline, draft, final); traces stored in Postgres |
| **FinOps** | Total pipeline cost must be <$0.05/run; budget limits enforced per agent and per day |

## Prerequisites

- **Docker Desktop** (with `docker compose` support)
- **OpenAI API key** (for LLM calls via LiteLLM proxy)
- **A modern shell** (bash, zsh, fish)
- **curl** (for health checks)

## Quick Start (4 Commands)

```bash
# 1. Clone the repository
git clone https://github.com/Clawdlinux/agentic-operator-core
cd examples/research-swarm

# 2. Create .env with your OpenAI key
echo "OPENAI_API_KEY=sk-proj-your-key-here" > .env

# 3. Start the stack
docker compose up -d

# 4. Run the pipeline
curl -X POST http://localhost:9000/orchestrate \
  -H "Content-Type: application/json" \
  -d '{"topic": "Climate adaptation finance 2026"}'
```

The pipeline will complete in ~90 seconds. You'll see a JSON response with:
- **Per-agent costs**: `"researcher": $0.0042, "writer": $0.0089, "editor": $0.0031`
- **Total cost**: ~$0.0183
- **Final artifact path**: MinIO bucket location
- **Trace ID**: For tracking this run

## What You'll See

### Cost Tracking Output (from orchestration response)

```json
{
  "trace_id": "a1b2c3d4-...",
  "total_duration_seconds": 92.4,
  "total_cost_usd": 0.0183,
  "stages": [
    {
      "stage": "researcher",
      "status": "completed",
      "cost_usd": 0.0042,
      "duration_seconds": 8.2
    },
    {
      "stage": "writer",
      "status": "completed",
      "cost_usd": 0.0089,
      "duration_seconds": 12.5
    },
    {
      "stage": "editor",
      "status": "completed",
      "cost_usd": 0.0052,
      "duration_seconds": 6.8
    }
  ],
  "minio_path": "demo-artifacts/a1b2c3d4-.../final_output.md",
  "final_output": "# Climate Adaptation Finance in 2026\n\n..."
}
```

### MinIO Artifacts

Visit [http://localhost:9090](http://localhost:9090) (MinIO console):
- **Login**: minioadmin / minioadmin123
- **Bucket**: `agentic-demo`
- **Artifacts**:
  - `demo-artifacts/<trace_id>/research_outline.json` – Research outline from researcher agent
  - `demo-artifacts/<trace_id>/draft.md` – Writer's first draft
  - `demo-artifacts/<trace_id>/final_output.md` – Editor's final polished version
  - `demo-artifacts/<trace_id>/final_artifact.json` – Full metadata

### Span Traces (Postgres)

Connect to Postgres at `localhost:5432` (user: `spans`, password: `spans_dev`):

```sql
SELECT trace_id, agent_role, operation, status, cost_usd, duration_ms
FROM spans_trace
ORDER BY created_at DESC
LIMIT 10;
```

## Architecture

### Docker Compose Stack

```
orchestrator (PORT 9000)
    ├─ researcher (PORT 9001)
    ├─ writer (PORT 9002)
    ├─ editor (PORT 9003)
    ├─ litellm-proxy (PORT 8000) ──────┬─── OpenAI API
    ├─ minio (PORT 9090, S3:9000) ─────┤
    └─ postgres (PORT 5432) ───────────┘
```

### Agent Pipeline

```
ORCHESTRATOR POST /orchestrate
    ↓
[RESEARCHER] POST /research
    • Takes: topic (str)
    • Returns: ResearchOutline JSON
    • Calls: LLM (gpt-4o-mini)
    • Models: 1 (gpt-4o-mini: $0.15/1M input, $0.60/1M output)
    ↓ (outline stored to MinIO)
[WRITER] POST /write
    • Takes: ResearchOutline, topic
    • Returns: Draft markdown (800-1200 words)
    • Calls: LLM (gpt-4o-mini)
    ↓ (draft stored to MinIO)
[EDITOR] POST /edit
    • Takes: Draft markdown
    • Returns: Final polished version + changelog JSON
    • Calls: LLM (gpt-4o-mini)
    ↓ (final + changelog stored to MinIO)
ORCHESTRATOR aggregates costs
    → Returns final output + cost breakdown
```

## Cost Breakdown

With current prompts and `gpt-4o-mini`:

| Stage | Model | Typical Tokens | Approx Cost |
|-------|-------|----------------|------------|
| Researcher | gpt-4o-mini | ~5,700 (input+output) | ~$0.0042 |
| Writer | gpt-4o-mini | ~5,300 | ~$0.0089 |
| Editor | gpt-4o-mini | ~2,700 | ~$0.0052 |
| **Total** | | **~13,700** | **~$0.0183** |

**Budget:** $0.10/agent/request enforced via LiteLLM virtual keys

## Customization

### Change Research Topic

```bash
curl -X POST http://localhost:9000/orchestrate \
  -H "Content-Type: application/json" \
  -d '{"topic": "Quantum computing breakthroughs 2026"}'
```

### Change Agent Models

Edit `config/litellm_config.yaml`:

```yaml
model_list:
  - model_name: gpt-4o-mini    # change to: gpt-4, gpt-4-turbo, etc
    litellm_params:
      model: openai/gpt-4o-mini
      api_key: ${OPENAI_API_KEY}
```

Then restart: `docker compose restart litellm-proxy`

### Change Agent Personas

Edit `docker-compose.yml` env vars:

```yaml
researcher:
  environment:
    AGENT_TONE: neutral_academic  # change to: conversational, technical, etc
```

### View Logs

```bash
# All services
docker compose logs -f

# Specific service
docker compose logs -f researcher
docker compose logs -f orchestrator
```

## Kubernetes Deployment (AKS Azure)

For production deployment on Azure Kubernetes Service (AKS) with Managed Identity and Azure Storage:

### 1. Prepare AKS Cluster

```bash
# Create resource group and AKS cluster
az group create --name agentic-demo --location westus2
az aks create --resource-group agentic-demo --name agentic-demo-aks \
  --enable-managed-identity --node-count 3

# Get credentials
az aks get-credentials --resource-group agentic-demo --name agentic-demo-aks
```

### 2. Set Up Azure Resources

```bash
# Create Storage Account for artifacts (replaces MinIO)
az storage account create --name agenticdemo --resource-group agentic-demo \
  --sku Standard_LRS

# Create Key Vault for secrets (replaces K8s Secrets)
az keyvault create --name agentic-demo-kv --resource-group agentic-demo

# Store OpenAI key in Key Vault
az keyvault secret set --vault-name agentic-demo-kv \
  --name openai-api-key --value "sk-proj-..."

# Create PostgreSQL for spans tracing
az postgres server create --resource-group agentic-demo \
  --name agentic-demo-db --admin-user spans --admin-password StrongPassword123
```

### 3. Configure AKS Workload Identity

```bash
# Enable workload identity on AKS
az aks update --resource-group agentic-demo --name agentic-demo-aks \
  --enable-oidc-issuer --enable-workload-identity

# Create service account with workload identity
kubectl apply -f k8s/azure-workload-identity/service-account.yaml

# Create Azure AD identity and grant permissions
az identity create --name agentic-demo --resource-group agentic-demo
```

### 4. Deploy to AKS

```bash
# Update K8s manifests with Azure-specific values
sed -i 's|agentic-agent:latest|acrname.azurecr.io/agentic-agent:latest|g' k8s/*.yaml

# Deploy Azure-specific namespace and secrets
kubectl apply -f k8s/azure-workload-identity/

# Deploy services
kubectl apply -f k8s/00-namespace.yaml
kubectl apply -f k8s/02-postgres.yaml
kubectl apply -f k8s/04-litellm.yaml
kubectl apply -f k8s/
```

### 5. Deploy Artifact Storage (Azure Blob)

Replace MinIO in production with Azure Blob Storage:

```python
# agents/azure_storage_client.py
from azure.storage.blob import BlobServiceClient
from azure.identity import ManagedIdentityCredential

credential = ManagedIdentityCredential()
blob_client = BlobServiceClient(
    account_url="https://<storage_account>.blob.core.windows.net",
    credential=credential
)

# Upload artifact
container = blob_client.get_container_client("agentic-demo")
container.upload_blob(name=minio_path, data=final_output, overwrite=True)
```

### Azure Cost Tracking (FinOps Integration)

Track total pipeline costs including:
- LLM API calls (via LiteLLM virtual keys)
- AKS compute (via Azure Monitor)
- Storage (Azure Blob)
- Database (PostgreSQL)

```bash
# View cost breakdown
az cost management query create \
  --resource-group agentic-demo \
  --timeframe MonthToDate \
  --granularity Daily
```

## Troubleshooting

### "docker compose: command not found"

Ensure Docker Desktop is installed with `docker compose` plugin:

```bash
docker compose version  # should show Docker Compose v2.x+
```

### "OPENAI_API_KEY not set"

```bash
# Add to .env
echo "OPENAI_API_KEY=sk-proj-your-actual-key" >> .env

# Or set in shell
export OPENAI_API_KEY=sk-proj-...
docker compose up
```

### Agent health checks failing

```bash
# Check logs
docker compose logs researcher
docker compose logs writer
docker compose logs editor

# Verify LiteLLM proxy started
curl http://localhost:8000/health
```

### High costs per run

- Check token usage in response metadata
- Reduce prompt verbosity in agent system prompts
- Use `gpt-4o-mini` instead of full `gpt-4`
- Shorter research topics → fewer tokens

## What's Next?

- **Integration**: Swap LiteLLM broker for your own LLM backend
- **Scaling**: Run on production Kubernetes (see AKS section above)
- **Custom Agents**: Add a 4th agent (reviewer, translator, etc)
- **Multi-Model**: Route researcher to gpt-4o, writer to gpt-4o-mini for cost optimization
- **User Feedback**: Add agent-level feedback loop for continuous improvement
- **Metrics**: Export traces to observability platform (Datadog, New Relic, etc)

## Support

For issues, questions, or feedback:
- GitHub Issues: https://github.com/Clawdlinux/agentic-operator-core/issues
- Documentation: https://github.com/Clawdlinux/agentic-operator-core/docs

---

**Total demo runtime**: ~90 seconds | **Total cost**: ~$0.02 | **Ready in under 10 minutes** ✅
