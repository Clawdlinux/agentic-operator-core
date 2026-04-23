# SRE Incident Response Demo
# Exercises: delegation, cost-aware routing, workloadType:kubernetes, adversarial tone

.PHONY: help build up down logs health run-demo clean demo

-include .env
COMPOSE_CMD = docker compose -f docker-compose.yml

help:
	@echo "SRE Incident Response Demo"
	@echo ""
	@echo "  ⚡ Quick Start:"
	@echo "    make demo               Run full incident response demo"
	@echo ""
	@echo "  Lifecycle:"
	@echo "    make build              Build Docker images"
	@echo "    make up / down          Start / stop stack"
	@echo "    make health             Service health check"
	@echo ""
	@echo "  Demo Scenarios:"
	@echo "    make run-demo                PodCrashLoopBackOff (default)"
	@echo "    make run-demo-oom            OOMKilled incident"
	@echo "    make run-demo-cpu            HighCPU incident"
	@echo "    make run-demo-node           NodeNotReady incident"
	@echo "    make run-demo-tool-block     Diagnostician tries kubectl_exec (blocked)"
	@echo "    make run-demo-budget         Remediator exceeds budget (OPA deny)"
	@echo ""
	@echo "  Operations:"
	@echo "    make logs               Tail all logs"
	@echo "    make costs              Per-agent cost breakdown"
	@echo "    make ps                 Container stats"
	@echo "    make clean              Remove everything"

build:
	$(COMPOSE_CMD) build lead-sre diagnostician remediator
	@echo "✅ Images built"

up: build
	$(COMPOSE_CMD) up -d
	@echo "⏳ Waiting for services..."
	@sleep 20
	@echo ""
	@echo "✅ SRE Incident Response stack is running!"
	@echo ""
	@echo "  🚨 Lead SRE:       http://localhost:9010"
	@echo "  🔬 Diagnostician:  http://localhost:9011"
	@echo "  🔧 Remediator:     http://localhost:9012"
	@echo "  🤖 LiteLLM:        http://localhost:8000"
	@echo ""
	@echo "Run 'make run-demo' to simulate an incident."

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
	@echo -n "  Lead SRE:      "; curl -sf http://localhost:9010/health | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('status','?'), '| mode:', d.get('collaboration_mode','?'), '| tier:', d.get('model_tier','?'))" 2>/dev/null || echo "❌ down"
	@echo -n "  Diagnostician: "; curl -sf http://localhost:9011/health | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('status','?'), '| tier:', d.get('model_tier','?'), '| memory:', d.get('memory_scope','?'))" 2>/dev/null || echo "❌ down"
	@echo -n "  Remediator:    "; curl -sf http://localhost:9012/health | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('status','?'), '| tier:', d.get('model_tier','?'), '| tone:', d.get('tone','?'))" 2>/dev/null || echo "❌ down"
	@echo ""

# ── Incident scenarios ──────────────────────────────────────────────────────

run-demo:
	@echo ""
	@echo "════════════════════════════════════════════════════════════"
	@echo "  🚨 SRE INCIDENT RESPONSE DEMO"
	@echo "  Mode: delegation | Strategy: cost-aware | Type: kubernetes"
	@echo "════════════════════════════════════════════════════════════"
	@echo ""
	@curl -s -X POST http://localhost:9010/incidents \
		-H "Content-Type: application/json" \
		-d '{"alert_type":"PodCrashLoopBackOff","namespace":"default","resource":"app-server-7b9f4c6d8-x2k9p"}' | python3 -m json.tool
	@echo ""

run-demo-oom:
	@echo "🚨 Incident: OOMKilled"
	@curl -s -X POST http://localhost:9010/incidents \
		-H "Content-Type: application/json" \
		-d '{"alert_type":"OOMKilled","namespace":"production","resource":"api-gateway-5d8c7f-m3j2k"}' | python3 -m json.tool

run-demo-cpu:
	@echo "🚨 Incident: HighCPUUtilization"
	@curl -s -X POST http://localhost:9010/incidents \
		-H "Content-Type: application/json" \
		-d '{"alert_type":"HighCPUUtilization","namespace":"staging","resource":"worker-pool-9a2b3c"}' | python3 -m json.tool

run-demo-node:
	@echo "🚨 Incident: NodeNotReady"
	@curl -s -X POST http://localhost:9010/incidents \
		-H "Content-Type: application/json" \
		-d '{"alert_type":"NodeNotReady","namespace":"kube-system","resource":"node-3"}' | python3 -m json.tool

run-demo-tool-block:
	@echo "⛔ Demo: Diagnostician tries restart_pod (NOT in diagnostician tool_profile)"
	@curl -s -X POST http://localhost:9011/invoke \
		-H "Content-Type: application/json" \
		-d '{"tool_name":"restart_pod","arguments":{"deployment":"app-server"},"confidence":0.99,"estimated_cost_usd":0}' | python3 -m json.tool

run-demo-budget:
	@echo "🚫 Demo: Remediator action exceeds $50 budget"
	@curl -s -X POST http://localhost:9012/invoke \
		-H "Content-Type: application/json" \
		-d '{"tool_name":"kubectl_exec","arguments":{"command":"drain node-3"},"confidence":0.99,"estimated_cost_usd":75.0}' | python3 -m json.tool

costs:
	@echo "Cost breakdown:"
	@echo -n "  Lead SRE:      "; curl -sf http://localhost:9010/costs | python3 -m json.tool 2>/dev/null || echo "unavailable"
	@echo -n "  Diagnostician: "; curl -sf http://localhost:9011/costs | python3 -m json.tool 2>/dev/null || echo "unavailable"
	@echo -n "  Remediator:    "; curl -sf http://localhost:9012/costs | python3 -m json.tool 2>/dev/null || echo "unavailable"

demo: up
	@sleep 5
	@$(MAKE) health
	@$(MAKE) run-demo

clean:
	$(COMPOSE_CMD) down --remove-orphans -v
	docker image rm -f $$(docker images --filter "reference=*sre-incident*" -q) 2>/dev/null || true
	@echo "✅ Cleaned up"
