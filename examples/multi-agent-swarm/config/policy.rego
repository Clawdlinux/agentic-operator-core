# OPA policy: strict mode budget enforcement
# Applied to every agent action before execution

package agentic.policy

import rego.v1

default allow := false

# Allow if cost is within per-agent budget
allow if {
    input.action.estimated_cost_usd <= input.policy.budget_limit_usd
}

# Allow if action has no cost (read-only)
allow if {
    input.action.estimated_cost_usd == 0
}

# Deny reason for budget violation
deny contains msg if {
    input.action.estimated_cost_usd > input.policy.budget_limit_usd
    msg := sprintf(
        "action exceeds per-agent budget limit ($%.2f > $%.2f max)",
        [input.action.estimated_cost_usd, input.policy.budget_limit_usd],
    )
}

# Deny if agent tries to use a tool outside its profile
deny contains msg if {
    count(input.agent.tool_profile) > 0
    not input.action.tool_name in input.agent.tool_profile
    msg := sprintf(
        "tool '%s' not in agent tool_profile %v",
        [input.action.tool_name, input.agent.tool_profile],
    )
}
