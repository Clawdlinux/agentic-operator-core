# Week 4: Critical Issues Remediation

## Overview
Address 8 CRITICAL issues identified in PR #11 code review (72 total issues).

## CRITICAL Fixes (Priority Order)

### C1: Nil-pointer Dereference on OPAPolicy Field
**File:** `pkg/controllers/agentworkload_controller.go`
**Issue:** Optional `OPAPolicy` field not checked before use
**Impact:** Panics on every reconcile when OPA is optional
**Fix:**
```go
// Add nil guard before accessing OPAPolicy
if spec.OPAPolicy != nil {
    // Use policy
}
```

### C2: Unregistered Webhook
**File:** `config/webhook/manifests.yaml` + controller setup
**Issue:** Validation/defaulting webhooks never registered
**Impact:** No webhook validation runs; C1 panics occur
**Fix:**
- Register ValidatingWebhook in manifests
- Add webhook server startup to main.go
- Update RBAC for webhook service account

### C3: SSRF Vulnerability in MCP Endpoint
**File:** `agents/tools/mcp_client.py`
**Issue:** User-controlled MCP endpoint not validated
**Impact:** Server-side request forgery attack surface
**Fix:**
- Add URL whitelist (allowed MCP hosts)
- Validate URL scheme (https only in production)
- Reject localhost/internal IPs

### C4+C5: Plaintext Credential Logging
**Files:** `agents/agents.py`, `agents/tools/litellm_client.py`
**Issue:** API keys logged in debug output
**Impact:** Secrets exposed in logs
**Fix:**
- Remove API key from log statements
- Use redaction filters
- Add credential sanitizer

### C6: Insecure Database Defaults
**File:** `agents/db/postgres_checkpoint.py`
**Issue:** No encryption, weak auth defaults
**Impact:** Production data at risk
**Fix:**
- Add SSL/TLS requirement
- Document credential management
- Add env var for sensitive config

### C7: Committed `.pyc` Files
**File:** `.gitignore` (missing entries)
**Issue:** Compiled Python bytecode in repo
**Impact:** Bloats repo, may cause version mismatch issues
**Fix:**
- Add `**/*.pyc` to `.gitignore`
- Remove existing `.pyc` files: `find . -name '*.pyc' -delete`
- Add to pre-commit hooks

### C8: Root Container Execution
**File:** `Dockerfile`
**Issue:** Container runs as root (UID 0)
**Impact:** Privilege escalation risk
**Fix:**
```dockerfile
RUN useradd -m -u 1000 agentrunner
USER agentrunner
```

## Implementation Tasks

- [ ] C1: Add OPAPolicy nil guards
- [ ] C2: Register and implement webhooks
- [ ] C3: Add MCP endpoint validation + URL whitelist
- [ ] C4+C5: Remove credential logging + add sanitizer
- [ ] C6: Add database security config
- [ ] C7: Clean .pyc files + update .gitignore
- [ ] C8: Add non-root user to Dockerfile
- [ ] Tests: Run full test suite
- [ ] Commit: Push fixes to GitHub

## Success Criteria
- ✅ All 8 CRITICAL issues resolved
- ✅ Tests pass (46/46)
- ✅ No security warnings
- ✅ Code review score >95%

## Notes
Using Kilo CLI (GLM-5 free) for code generation + Opus for code review.
