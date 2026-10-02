#!/bin/bash
# Code Review Script - Reviews Task code using GLM5 (Kilo)
# Usage: ./scripts/review_code.sh [task_name]

set -e

TASK=${1:-"WEEK2_TASK2A"}
REVIEW_PROMPT_FILE="/tmp/code_review_prompt_${TASK}.txt"
REVIEW_OUTPUT_FILE="/tmp/code_review_${TASK}.md"

echo "=== Code Review: $TASK ==="
echo "Model: GLM5 (Kilo, free)"
echo ""

# Collect code files based on task
case $TASK in
  "WEEK2_TASK2A")
    echo "Reviewing: Webhook Validation + Tests"
    FILES=(
      "api/v1alpha1/agentworkload_webhook.go"
      "api/v1alpha1/agentworkload_webhook_test.go"
      "config/webhook/validating_webhook.yaml"
    )
    ;;
  "WEEK2_TASK2B")
    echo "Reviewing: OPA Policies"
    FILES=(
      "pkg/rules/threshold/policies.rego"
      "pkg/rules/threshold/policies_test.go"
    )
    ;;
  "WEEK2_TASK2C")
    echo "Reviewing: OPA Evaluator"
    FILES=(
      "pkg/rules/threshold/evaluator.go"
      "pkg/rules/threshold/evaluator_test.go"
    )
    ;;
  *)
    echo "Unknown task: $TASK"
    exit 1
    ;;
esac

# Build the review prompt
{
  cat <<'EOF'
# Code Review Request - Automated Review using Code Review Skill

You are a code reviewer. Review the following code files systematically using these dimensions:

1. **Security** (CRITICAL)
   - Input validation
   - Authentication/Authorization
   - Secrets handling
   - Dependency safety

2. **Correctness** (HIGH)
   - Logic errors
   - Edge cases
   - Error handling
   - Boundary conditions

3. **Performance** (HIGH)
   - Efficiency
   - Resource usage
   - Unnecessary computation

4. **Maintainability** (MEDIUM)
   - Code clarity
   - Single responsibility
   - DRY principle
   - Naming quality

5. **Testing** (MEDIUM)
   - Test coverage
   - Edge case testing
   - Test quality

For each issue found, report:
- Severity: CRITICAL | HIGH | MEDIUM | LOW
- Location: file:line or file
- Issue: Clear description
- Fix: Recommended fix
- Pattern: If reusable learning

End with a PASS/FAIL decision.

---

## Code Files to Review

EOF

  for file in "${FILES[@]}"; do
    if [ -f "$file" ]; then
      echo "### File: $file"
      echo '```'
      cat "$file"
      echo '```'
      echo ""
    else
      echo "⚠️  File not found: $file"
    fi
  done
} > "$REVIEW_PROMPT_FILE"

echo "Prompt file: $REVIEW_PROMPT_FILE"
echo "Sending to GLM5 for review..."
echo ""

# Call GLM5 via OpenClaw session (use gemini skill with GLM model)
# Since we don't have direct API access here, we'll use a marker for manual review
cat <<EOF

=== REVIEW PROMPT READY ===

Files included:
$(printf "  - %s\n" "${FILES[@]}")

To run automated review using GLM5:
  gemini @${REVIEW_PROMPT_FILE}

Or for Kimi K2.5:
  /model kimi
  gemini @${REVIEW_PROMPT_FILE}

Output will be written to: $REVIEW_OUTPUT_FILE

=== Ready for review ===
EOF
