# OPA policy: SRE incident response budget + blast radius

package agentic.sre_policy

import rego.v1

default allow := false

# Allow if cost within budget
allow if {
    input.action.estimated_cost_usd <= input.policy.budget_limit_usd
}

# Allow read-only actions always
allow if {
    input.action.tool_name in {"log_search", "metric_query", "trace_lookup", "triage_alert", "compile_report"}
}

# Deny destructive actions above budget
deny contains msg if {
    input.action.estimated_cost_usd > input.policy.budget_limit_usd
    msg := sprintf(
        "remediation cost exceeds budget ($%.2f > $%.2f max)",
        [input.action.estimated_cost_usd, input.policy.budget_limit_usd],
    )
}

# Deny if agent tries tools outside its profile
deny contains msg if {
    count(input.agent.tool_profile) > 0
    not input.action.tool_name in input.agent.tool_profile
    msg := sprintf(
        "tool '%s' not in agent tool_profile %v",
        [input.action.tool_name, input.agent.tool_profile],
    )
}

# Warn on destructive operations in non-prod namespaces
warn contains msg if {
    input.action.tool_name in {"restart_pod", "scale_deployment", "kubectl_exec"}
    input.action.namespace == "production"
    msg := "destructive operation in production namespace — extra review required"
}
