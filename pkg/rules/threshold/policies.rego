package agentworkload.policies

# OPA Policy Rules for AgentWorkload Action Execution
# Generic, tool-agnostic safety policies that apply to any infrastructure (Ceph, MinIO, PostgreSQL, etc.)

# ===================================================================
# RULE 1: ALLOW high-confidence actions
# ===================================================================
# If confidence >= 0.95, allow execution immediately
allow {
    input.action_type != "destructive"
    input.confidence >= 0.95
    input.cluster_health_score >= 50
}

# ===================================================================
# RULE 2: DENY low-confidence actions (require human approval)
# ===================================================================
# If confidence < 0.95, deny and require human review
deny[msg] {
    input.confidence < 0.95
    msg := sprintf("Low confidence action (%.2f) requires human approval. Threshold: 0.95", [input.confidence])
}

# ===================================================================
# RULE 3: DENY destructive operations without extremely high confidence
# ===================================================================
# Actions like delete/remove/purge require confidence >= 0.99
deny[msg] {
    is_destructive_action(input.action_type)
    input.confidence < 0.99
    msg := sprintf("Destructive action '%s' requires confidence >= 0.99, got %.2f", [input.action_type, input.confidence])
}

# ===================================================================
# RULE 4: DENY any action during cluster degradation
# ===================================================================
# If cluster health is low (<50%), only allow read-only operations
deny[msg] {
    input.cluster_health_score < 50
    not is_readonly_action(input.action_type)
    msg := sprintf("Cluster health is degraded (%.1f%%). Only read-only operations allowed. Requested: %s", [input.cluster_health_score, input.action_type])
}

# ===================================================================
# RULE 5: ALLOW read-only operations unconditionally
# ===================================================================
# Monitor, get, list, describe operations are always safe
allow {
    is_readonly_action(input.action_type)
}

# ===================================================================
# RULE 6: DENY when multiple safety rules violated
# ===================================================================
# If both low confidence AND degraded health, extra emphasis in denial
deny[msg] {
    input.confidence < 0.90
    input.cluster_health_score < 40
    msg := "CRITICAL: Low confidence AND cluster degradation. Manual intervention required."
}

# ===================================================================
# HELPER FUNCTIONS
# ===================================================================

# is_destructive_action returns true if action type is destructive
is_destructive_action(action_type) {
    destructive := ["delete", "remove", "purge", "drop", "reset", "cleanup", "clear"]
    action_type_lower := lower(action_type)
    destructive[_] == action_type_lower
}

# is_readonly_action returns true if action is read-only
is_readonly_action(action_type) {
    readonly := ["get", "list", "describe", "monitor", "analyze", "read", "check", "validate"]
    action_type_lower := lower(action_type)
    readonly[_] == action_type_lower
}

# ===================================================================
# DECISION OUTPUT
# ===================================================================

# Final decision: allow if no deny rules match
final_decision := "ALLOW" {
    count(deny) == 0
    allow
}

# Final decision: deny if any deny rule matches
final_decision := "DENY" {
    count(deny) > 0
}

# Deny reasons for output
deny_reasons[msg] {
    deny[msg]
}

# Confidence level assessment
confidence_level := "HIGH" {
    input.confidence >= 0.95
}

confidence_level := "MEDIUM" {
    input.confidence >= 0.8
    input.confidence < 0.95
}

confidence_level := "LOW" {
    input.confidence < 0.8
}

# Action category assessment
action_category := "DESTRUCTIVE" {
    is_destructive_action(input.action_type)
}

action_category := "READONLY" {
    is_readonly_action(input.action_type)
}

action_category := "MODIFICATION" {
    not is_destructive_action(input.action_type)
    not is_readonly_action(input.action_type)
}

# Cluster health assessment
cluster_status := "HEALTHY" {
    input.cluster_health_score >= 80
}

cluster_status := "DEGRADED" {
    input.cluster_health_score >= 50
    input.cluster_health_score < 80
}

cluster_status := "CRITICAL" {
    input.cluster_health_score < 50
}

# ===================================================================
# AUDIT TRAIL
# ===================================================================

audit_entry := {
    "action": input.action_type,
    "confidence": input.confidence,
    "confidence_level": confidence_level,
    "cluster_health": input.cluster_health_score,
    "cluster_status": cluster_status,
    "action_category": action_category,
    "decision": final_decision,
    "reasons": deny_reasons,
    "evaluated_at": "runtime"
}
