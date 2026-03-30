# Quick reference for common commands

.PHONY: help build up down logs clean cost-report

help:
	@echo "Research Swarm Demo - Make Commands"
	@echo ""
	@echo "Setup & Lifecycle:"
	@echo "  make build              Build Docker images (agent, orchestrator)"
	@echo "  make up                 Start all services (docker compose)"
	@echo "  make down               Stop all services"
	@echo "  make restart            Restart all services"
	@echo "  make clean              Remove containers, images, volumes (WARNING: destructive)"
	@echo ""
	@echo "Inspection & Debugging:"
	@echo "  make logs               Tail all service logs"
	@echo "  make logs-service SVC=<name>  Tail specific service(researcher|writer|editor|orchestrator|litellm-proxy)"
	@echo "  make health             Check health of all services"
	@echo "  make ps                 Show running containers, memory usage"
	@echo ""
	@echo "Testing:"
	@echo "  make run-demo           Run orchestration demo (single request)"
	@echo "  make cost-report        Show per-agent cost usage (from litellm proxy)"
	@echo "  make minio-console      Print MinIO console URL"
	@echo "  make postgres-shell     Connect to PostgreSQL shell (requires psql installed)"
	@echo ""
	@echo "Setup:"
	@echo "  make env                Create .env file (prompts for OPENAI_API_KEY)"
	@echo ""

build:
	docker build -f docker/Dockerfile.agent -t agentic-agent:latest .
	docker build -f docker/Dockerfile.orchestrator -t agentic-orchestrator:latest .
	@echo "✅ Images built: agentic-agent:latest, agentic-orchestrator:latest"

up:
	docker compose -f docker-compose.yml up -d
	@echo "✅ Stack started. Waiting for services..."
	@sleep 10
	@echo "📊 Orchestrator:  http://localhost:9000"
	@echo "📦 MinIO console: http://localhost:9090 (minioadmin / minioadmin123)"
	@echo "🗄️  Postgres:      localhost:5432 (spans / spans_dev)"
	@echo ""
	@echo "Run 'make run-demo' to start a pipeline."

down:
	docker compose -f docker-compose.yml down
	@echo "✅ Stack stopped"

restart:
	docker compose -f docker-compose.yml restart
	@echo "✅ Stack restarted"

logs:
	docker compose -f docker-compose.yml logs -f

logs-service:
	docker compose -f docker-compose.yml logs -f $(SVC)

clean:
	@echo "⚠️  WARNING: This will remove ALL containers, images, and volumes."
	@read -p "Are you sure? (y/N) " -n 1 -r; \
	echo; \
	if [[ $$REPLY =~ ^[Yy]$$ ]]; then \
		docker compose -f docker-compose.yml down -v; \
		docker rmi agentic-agent:latest agentic-orchestrator:latest || true; \
		echo "✅ Cleaned"; \
	else \
		echo "Cancelled"; \
	fi

health:
	@echo "Checking health endpoints..."
	@curl -s http://localhost:9001/health && echo "✅ researcher" || echo "❌ researcher"
	@curl -s http://localhost:9002/health && echo "✅ writer" || echo "❌ writer"
	@curl -s http://localhost:9003/health && echo "✅ editor" || echo "❌ editor"
	@curl -s http://localhost:9000/health && echo "✅ orchestrator" || echo "❌ orchestrator"
	@curl -s http://localhost:8000/health && echo "✅ litellm-proxy" || echo "❌ litellm-proxy"

ps:
	docker stats --no-stream

run-demo:
	@echo "Running orchestration demo..."
	curl -s -X POST http://localhost:9000/orchestrate \
		-H "Content-Type: application/json" \
		-d '{"topic": "Quantum computing breakthroughs in 2026"}' | jq .
	@echo ""

cost-report:
	@echo "Per-Agent Cost Report:"
	@echo "Researcher (sk-researcher-virtual):"
	@curl -s 'http://localhost:8000/spend/logs?virtual_key=sk-researcher-virtual' | jq '.data[] | {timestamp, total_cost: .cost}' 2>/dev/null || echo "  (no data yet)"
	@echo ""
	@echo "Writer (sk-writer-virtual):"
	@curl -s 'http://localhost:8000/spend/logs?virtual_key=sk-writer-virtual' | jq '.data[] | {timestamp, total_cost: .cost}' 2>/dev/null || echo "  (no data yet)"
	@echo ""
	@echo "Editor (sk-editor-virtual):"
	@curl -s 'http://localhost:8000/spend/logs?virtual_key=sk-editor-virtual' | jq '.data[] | {timestamp, total_cost: .cost}' 2>/dev/null || echo "  (no data yet)"

minio-console:
	@echo "MinIO Console: http://localhost:9090"
	@echo "Login: minioadmin / minioadmin123"
	@echo "Bucket: agentic-demo"

postgres-shell:
	psql postgresql://spans:spans_dev@localhost:5432/spans

env:
	@if [ -f .env ]; then \
		echo ".env already exists"; \
	else \
		echo "Creating .env file..."; \
		read -p "Enter OpenAI API Key (sk-proj-...): " key; \
		echo "OPENAI_API_KEY=$$key" > .env; \
		echo "✅ .env created"; \
	fi
