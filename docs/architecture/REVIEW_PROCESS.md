# Code Review Process - Agentic Operator

**Requirement:** All code must be reviewed by GLM5 (free model) before GitHub push.

---

## Workflow

### Step 1: Prepare Changes
```bash
# Make your changes
git add .
git commit -m "Feature: ..."
```

### Step 2: Run Manual Review
```bash
# Before pushing, run:
./scripts/review-before-push.sh
```

This generates a review request with all changed files and opens them for review.

### Step 3: Address Review Issues
- **CRITICAL issues:** MUST be fixed before push
- **HIGH issues:** MUST be fixed before push
- **MEDIUM issues:** Should be fixed before push
- **LOW issues:** Can be fixed in follow-up PR

### Step 4: Push with Confidence
```bash
# After review passes, push:
git push origin main

# Or skip review if confident (NOT RECOMMENDED):
git push origin main --no-verify
```

---

## Review Dimensions (Systematic)

### SECURITY (CRITICAL)
- No hardcoded secrets or API keys
- No SQL injection, XSS, or command injection
- Input validation and error handling
- Proper authorization and authentication
- Secrets managed via Kubernetes Secrets only

### CORRECTNESS (HIGH)
- Logic is correct and handles edge cases
- Null/nil checks in place
- Comprehensive error handling
- No off-by-one or boundary errors
- Thread-safe state mutations where needed

### PERFORMANCE (HIGH)
- No N+1 queries or unnecessary loops
- Efficient memory usage
- Reasonable timeouts and limits
- No blocking operations where async available

### MAINTAINABILITY (MEDIUM)
- Code is clear and well-documented
- Follows project conventions
- Single responsibility principle
- No magic numbers or constants
- DRY principle followed

### TESTING (MEDIUM)
- Changes covered by tests
- Comprehensive test cases (positive + negative)
- Mock objects used correctly
- Clear test names
- Edge cases tested

---

## Review Checklist Template

```
SECURITY
- [ ] No secrets hardcoded
- [ ] Input properly validated
- [ ] Error messages don't leak sensitive info
- [ ] RBAC/authorization enforced

CORRECTNESS
- [ ] Logic handles all edge cases
- [ ] Error handling comprehensive
- [ ] Null/nil checks in place
- [ ] State mutations safe

PERFORMANCE
- [ ] No N+1 patterns
- [ ] Memory efficient
- [ ] Timeouts reasonable
- [ ] No blocking operations

MAINTAINABILITY
- [ ] Code is clear
- [ ] Follows conventions
- [ ] No duplication
- [ ] Well-commented

TESTING
- [ ] Changes covered
- [ ] Tests comprehensive
- [ ] Mocking done right
- [ ] Edge cases tested

DECISION: ✅ PASS | ⚠️ CONDITIONAL | ❌ FAIL
```

---

## Using GLM5 for Automated Review

GLM5 is **free** and fast enough for code review:

```bash
# Manual invocation (requires gemini CLI):
gemini "Review this code: $(cat file.go)"

# Via the pre-commit hook (automatic):
git push
# Hook runs review before push
```

---

## GitHub Branch Protection (Optional)

To enforce reviews at the GitHub level:

1. Go to repo Settings → Branches → Branch Protection
2. Enable: "Require code reviews before merging"
3. Require approvals: 1
4. Dismiss stale reviews: ON
5. Require status checks to pass: ON

---

## Bypassing Reviews (When Needed)

For urgent hotfixes or documentation changes:

```bash
# Skip the pre-push hook:
git push --no-verify
```

⚠️ **Not recommended** — only for genuine emergencies.

---

## Examples

### Week 2 Code Review (Model)
```
CRITICAL ISSUES (3):
1. Type assertion panics (file.go:109)
   - Severity: CRITICAL
   - Fix: Use comma-ok idiom
   
2. OPA package declaration (policies.rego:1)
   - Severity: CRITICAL
   - Fix: Change to package agentworkload.policies

3. Policy mode ignored (controller.go:125)
   - Severity: CRITICAL
   - Fix: Call EvaluateStrict() or EvaluatePermissive()

HIGH ISSUES (4):
... [similar format]

MEDIUM ISSUES (5):
... [similar format]

DECISION: ❌ FAIL
Fix all CRITICAL issues before merge.
```

### Week 3 Code Review (This Session)
```
CHANGES: 19 new tests, 1,940 LOC Python
FILES: state.py, browserless.py, litellm_client.py, workflow.py

SECURITY:
✅ No secrets hardcoded
✅ API keys via Kubernetes Secrets
✅ Input validation in place

CORRECTNESS:
✅ Error handling comprehensive
✅ Timeout management (30s)
✅ Edge cases handled

PERFORMANCE:
✅ No N+1 patterns
✅ Efficient async/await
✅ Reasonable limits

MAINTAINABILITY:
✅ Clear code structure
✅ Follows conventions
✅ Well-documented

TESTING:
✅ 19/19 tests passing
✅ Mocks used appropriately
✅ Edge cases covered

DECISION: ✅ PASS
Ready to merge.
```

---

## Questions?

If review is unclear:
1. Check REVIEW_DIMENSIONS above
2. Compare to similar code in codebase
3. Ask for clarification in review comments
4. Escalate to code-review skill for detailed analysis

**Default:** When in doubt, fix it. Better safe than sorry.
