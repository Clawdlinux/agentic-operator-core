package agentic.policies.model

default allow = false

# Model allow-list policy.
#
# Inputs:
# - provider
# - model
# - allowed_providers: [string]
# - allowed_models: [string]

provider := object.get(input, "provider", "")
model := object.get(input, "model", "")

allowed_providers := {p | p := object.get(input, "allowed_providers", [])[_]}
allowed_models := {m | m := object.get(input, "allowed_models", [])[_]}

provider_allowed {
    provider != ""
    allowed_providers[provider]
}

model_allowed {
    model != ""
    allowed_models[model]
}

allow {
    provider_allowed
    model_allowed
}

deny[msg] {
    not provider_allowed
    msg := sprintf("Provider '%s' is not allow-listed", [provider])
}

deny[msg] {
    not model_allowed
    msg := sprintf("Model '%s' is not allow-listed", [model])
}

decision := {
    "allow": allow,
    "provider": provider,
    "model": model,
    "reasons": [msg | deny[msg]],
}
