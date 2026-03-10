# Agentic Kubernetes Operator

Kubernetes operator for orchestrating tool-agnostic AI agent workloads via `AgentWorkload` CRDs and MCP-compatible tool backends.

## For VCs & Reviewers

- We are building infrastructure for autonomous, policy-aware agent workflows on Kubernetes.
- This public repo highlights product behavior through live demo videos and concrete use cases.
- Start with the Operator Demo link below, then the Full Walkthrough.
- Private sales collateral (including the pitch deck) is intentionally not published here.

## 🎬 Demo

### Operator Demo

[▶ Watch Operator Demo (Streaming)](https://cdn.jsdelivr.net/gh/Clawdlinux/agentic-operator-core@main/agentic-operator-demo.mp4)

### Full Walkthrough

[▶ Watch Full Walkthrough (Streaming)](https://cdn.jsdelivr.net/gh/Clawdlinux/agentic-operator-core@main/demo-video.mp4)

Direct file links:
- [agentic-operator-demo.mp4](https://cdn.jsdelivr.net/gh/Clawdlinux/agentic-operator-core@main/agentic-operator-demo.mp4)
- [demo-video.mp4](https://cdn.jsdelivr.net/gh/Clawdlinux/agentic-operator-core@main/demo-video.mp4)

## ✅ Use Cases

- Competitive intelligence pipelines that gather and summarize market signals
- Autonomous Kubernetes remediation workflows with policy checks
- Multi-agent research workflows that coordinate analysis and reporting

## 🚀 Quick Start

```bash
git clone https://github.com/Clawdlinux/agentic-operator-core
cd agentic-operator-core

# examples
kubectl apply -f config/agentworkload_example.yaml
```

For full installation and runtime configuration, see the docs in [docs](docs).

## Public / Private Boundary

This repository is intentionally open-source friendly and excludes private sales material.
The pitch deck is maintained in private channels and is not published in this public repo.

## Contributing

- Open an issue for bugs or feature proposals
- Submit PRs against `main`
- Keep changes scoped and testable

## License

Apache License 2.0. See [LICENSE](LICENSE).
