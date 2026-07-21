# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 0.2.x   | Supported |
| 0.1.x   | Best effort |

## Reporting a Vulnerability

If you discover a security vulnerability in agentic-operator, please report it
responsibly. **Do not open a public GitHub issue.**

### How to report

1. Use [GitHub private vulnerability reporting](https://github.com/Clawdlinux/agentic-operator-core/security/advisories/new).
2. Include steps to reproduce, affected versions, and potential impact.
3. We aim to acknowledge receipt within 2 business days.
4. Fix and disclosure timelines depend on severity, exploitability, and coordinated disclosure needs.

### What to expect

- A private acknowledgement through GitHub Security Advisories.
- Updates when triage status or disclosure timing changes.
- Credit in the release notes (unless you prefer anonymity).

### Scope

The following are in scope:

- Kubernetes RBAC escalation via the operator
- Webhook bypass or admission validation flaws
- Secret leakage (credentials, tokens, keys)
- Container escape or privilege escalation
- Injection attacks via CRD fields

### Out of Scope

- Vulnerabilities in upstream dependencies (report to the upstream project)
- Denial-of-service via resource exhaustion (use ResourceQuota/LimitRange)
- Issues requiring physical access to the cluster nodes
