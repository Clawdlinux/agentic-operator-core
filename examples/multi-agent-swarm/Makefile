# Multi-Agent Swarm Demo
# Proves: A2A, persona tool_profile, autoApproveThreshold, OPA, collaborationMode:team

.PHONY: help build up down logs health run-demo clean demo verify

COMPOSE_CMD = docker compose -f docker-compose.yml

help:
	@echo "Multi-Agent Swarm Demo"
	@echo ""
	@echo "  ⚡ Quick Start:"
	@echo "    make demo               Run entire demo end-to-end"
	@echo ""
	@echo "  Lifecycle:"
	@echo "    make build              Build Docker images"
	@echo "    make up / down          Start / stop stack"
	@echo "    make health             Service health check"
	@echo ""
	@echo "  Demo:"
	@echo "    make run-demo           Send /analyze request"
	@echo "    make run-demo-tool-block   Demo persona tool blocking"
	@echo "    make run-demo-approval     Demo approval threshold gating"
	@echo "    make run-demo-opa          Demo OPA budget enforcement"
	@echo ""
	@echo "  Operations:"
	@echo "    make logs               Tail all logs"
	@echo "    make ps                 Container stats"
	@echo "    make clean              Remove everything"
	@echo ""

build:
	$(COMPOSE_CMD) build analyst strategist orchestrator
	@echo "✅ Images built"

up: build
	$(COMPOSE_CMD) up -d
	@echo "⏳ Waiting for services to be healthy..."
	@sleep 15
	@echo ""
	@echo "✅ Multi-Agent Swarm is running!"
	@echo ""
	@echo "  📊 Orchestrator:    http://localhost:9000"
	@echo "  🔬 Analyst Agent:   http://localhost:9001"
	@echo "  📈 Strategist Agent: http://localhost:9002"
	@echo "  🤖 LiteLLM Proxy:  http://localhost:8000"
	@echo "  📦 MinIO Console:   http://localhost:9090"
	@echo "  🗄️  Postgres:       localhost:5432"
	@echo ""
	@echo "Run 'make run-demo' to start the demo."

down:
	$(COMPOSE_CMD) down --remove-orphans
	@echo "✅ Stack stopped"

logs:
	$(COMPOSE_CMD) logs -f

ps:
	$(COMPOSE_CMD) ps

health:
	@echo "Checking services..."
	@echo ""
	@echo -n "  LiteLLM:     "; curl -sf http://localhost:8000/health | python3 -c "import sys,json; print(json.load(sys.stdin).get('status','unknown'))" 2>/dev/null || echo "❌ down"
	@echo -n "  Analyst:     "; curl -sf http://localhost:9001/health | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('status','?'), '| tools:', d.get('tool_profile','?'))" 2>/dev/null || echo "❌ down"
	@echo -n "  Strategist:  "; curl -sf http://localhost:9002/health | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('status','?'), '| tools:', d.get('tool_profile','?'))" 2>/dev/null || echo "❌ down"
	@echo -n "  Orchestrator:"; curl -sf http://localhost:9000/health | python3 -c "import sys,json; print(json.load(sys.stdin).get('status','unknown'))" 2>/dev/null || echo "❌ down"
	@echo -n "  Postgres:    "; pg_isready -h localhost -p 5432 -U spans 2>/dev/null && echo "✅ ready" || echo "❌ down"
	@echo ""

# ── Full demo: all 6 governance stages ──────────────────────────────────────

run-demo:
	@echo ""
	@echo "════════════════════════════════════════════════════════════"
	@echo "  MULTI-AGENT SWARM DEMO"
	@echo "  Proves: A2A, Persona, Approval, OPA, Team Collaboration"
	@echo "════════════════════════════════════════════════════════════"
	@echo ""
	@curl -s -X POST http://localhost:9000/analyze \
		-H "Content-Type: application/json" \
		-d '{"topic": "AI agent infrastructure market"}' | python3 -m json.tool
	@echo ""
	@echo "════════════════════════════════════════════════════════════"
	@echo "  ✅ Demo complete! Check 'make logs' for detailed output."
	@echo "════════════════════════════════════════════════════════════"

# ── Individual governance demos ─────────────────────────────────────────────

run-demo-tool-block:
	@echo "⛔ Demo: Analyst tries execute_trade (blocked by persona tool_profile)"
	@curl -s -X POST http://localhost:9001/invoke \
		-H "Content-Type: application/json" \
		-d '{"tool_name":"execute_trade","arguments":{"action":"buy","amount_usd":1000},"confidence":0.99,"estimated_cost_usd":0.0}' | python3 -m json.tool

run-demo-approval:
	@echo "⏸️  Demo: Strategist low-confidence action (held by autoApproveThreshold)"
	@curl -s -X POST http://localhost:9002/invoke \
		-H "Content-Type: application/json" \
		-d '{"tool_name":"execute_trade","arguments":{"action":"buy","amount_usd":5},"confidence":0.82,"estimated_cost_usd":5.0}' | python3 -m json.tool

run-demo-opa:
	@echo "🚫 Demo: Strategist $50 trade (denied by OPA budget limit $25)"
	@curl -s -X POST http://localhost:9002/invoke \
		-H "Content-Type: application/json" \
		-d '{"tool_name":"execute_trade","arguments":{"action":"buy","amount_usd":50},"confidence":0.99,"estimated_cost_usd":50.0}' | python3 -m json.tool

# ── Full demo script ────────────────────────────────────────────────────────

demo: up
	@sleep 5
	@$(MAKE) health
	@$(MAKE) run-demo

verify: health

clean:
	$(COMPOSE_CMD) down --remove-orphans -v
	docker image rm -f $$(docker images --filter "reference=*multi-agent-swarm*" -q) 2>/dev/null || true
	@echo "✅ Cleaned up"
