# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 0.1.x   | :white_check_mark: |

## Reporting a Vulnerability

If you discover a security vulnerability in agentic-operator, please report it
responsibly. **Do not open a public GitHub issue.**

### How to report

1. Email **security@clawdlinux.org** with a description of the vulnerability.
2. Include steps to reproduce, affected versions, and potential impact.
3. We will acknowledge receipt within **48 hours**.
4. We aim to provide a fix or mitigation within **7 days** for critical issues.

### What to expect

- A confirmation email within 48 hours.
- Regular updates on the status of your report.
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
