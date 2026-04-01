package agentic.policies.egress

default allow = false

# Egress domain policy for workload safety.
#
# Inputs:
# - requested_egress_domains: [string]
# - allowed_egress_domains: [string]

requested := object.get(input, "requested_egress_domains", [])
allowed := {d | d := object.get(input, "allowed_egress_domains", [])[_]}

violations[domain] {
    domain := requested[_]
    not allowed[domain]
}

allow {
    count(violations) == 0
}

deny[msg] {
    count(violations) > 0
    msg := sprintf("Disallowed egress domains requested: %v", [[d | violations[d]]])
}

decision := {
    "allow": allow,
    "requested_domains": requested,
    "denied_domains": [d | violations[d]],
    "reasons": [msg | deny[msg]],
}
