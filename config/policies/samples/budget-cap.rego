package agentic.policies.budget

default allow = false

# Budget policy for workload execution approval.
#
# Inputs:
# - budget_cap_usd
# - spend_month_to_date_usd
# - estimated_cost_usd

effective_budget := object.get(input, "budget_cap_usd", 0)
spend_mtd := object.get(input, "spend_month_to_date_usd", 0)
estimated_cost := object.get(input, "estimated_cost_usd", 0)
projected_total := spend_mtd + estimated_cost

allow {
    effective_budget > 0
    projected_total <= effective_budget
}

deny[msg] {
    effective_budget <= 0
    msg := "budget_cap_usd must be set to a positive value"
}

deny[msg] {
    projected_total > effective_budget
    msg := sprintf(
        "Budget exceeded: projected %.2f USD exceeds cap %.2f USD",
        [projected_total, effective_budget],
    )
}

decision := {
    "allow": allow,
    "projected_total_usd": projected_total,
    "budget_cap_usd": effective_budget,
    "reasons": [msg | deny[msg]],
}
