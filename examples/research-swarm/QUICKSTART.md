# Research Swarm Demo - 10-Minute Quick Start

## Setup (2 minutes)

```bash
# Copy example environment
cp .env.example .env

# Edit .env and add your OpenAI API key
# Required: OPENAI_API_KEY=sk-proj-your-key-here
# Recommended: set strong MINIO_ROOT_PASSWORD and POSTGRES_PASSWORD

# Build Docker images
make build

# Start the stack
make up
```

## Run Demo (3 minutes)

```bash
# 1. Check health
make health
# Expected: ✅ researcher, ✅ writer, ✅ editor, ✅ orchestrator

# 2. Run orchestration
make run-demo
# Runs a single research pipeline on "Quantum computing breakthroughs in 2026"

# 3. View results
# - Check orchestrator response (JSON) for:
#   - total_cost_usd (should be ~$0.02)
#   - Per-stage costs and durations
#   - minio_path (location of final artifact)
```

## Inspect (3 minutes)

```bash
# View all logs
make logs

# View specific service
make logs-service SVC=researcher

# Check costs per agent
make cost-report

# Open MinIO console
make minio-console
# Then: Bucket → agentic-demo → trace_ids → your_trace_id
```

## Cleanup

```bash
make down          # Stop without removing data
make clean         # Stop + remove everything (WARNING)
```

## Customization

Edit these to customize behavior:

| File | What | How |
|------|------|-----|
| `docker-compose.yml` | Agent env vars | Change `AGENT_ROLE`, `AGENT_TONE`, topics |
| `config/litellm_config.yaml` | LLM models | Switch model_name to `gpt-4`, `gpt-4-turbo`, etc |
| `agents/researcher/main.py` | Research logic | Add real web_search, modify prompts |
| `agents/writer/main.py` | Writing logic | Change prose style, length, format |
| `agents/editor/main.py` | Editing logic | Modify quality criteria, fact-check |

## Cost Tracking Explained

Each agent has a LiteLLM **virtual key** that isolates spending:
- `sk-researcher-virtual` (limit: $20/day)
- `sk-writer-virtual` (limit: $30/day)
- `sk-editor-virtual` (limit: $20/day)

LiteLLM tracks each key's usage. Run `make cost-report` to see real costs.

## Kubernetes Deployment

For production (Azure AKS), see README.md section "Kubernetes Deployment (AKS Azure)".

---

**Total time: ~10 minutes** | **Total cost: ~$0.02** | **Next: `make run-demo`** ✅
