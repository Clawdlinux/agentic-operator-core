{{/*
Security render-time guards.

These templates enforce the NineVigil posture documented in
docs/security/threat-model.md:

  - No plaintext credentials in values.yaml (subchart or umbrella)
  - No unrestricted egress from agent namespaces
  - No opt-in to external-SaaS OAuth unless explicitly flagged

They run at `helm template` / `helm install` time. A failed guard aborts
render with a descriptive error, so a misconfigured cluster never reaches
the API server.
*/}}

{{/*
Assert: no plaintext credentials in values.

Triggered when security.secrets.requireExistingSecret is true (default).
Each subchart that carries a credential must either leave the plaintext
field empty (auto-generate or no-op) or reference an existingSecret.
*/}}
{{- define "agentic-operator.assertNoPlaintextSecrets" -}}
{{- if .Values.security.secrets.requireExistingSecret -}}
  {{- $errs := list -}}

  {{- if .Values.litellm.enabled -}}
    {{- if and (ne (.Values.litellm.openaiKey | default "") "") (eq (.Values.litellm.existingSecret | default "") "") -}}
      {{- $errs = append $errs "litellm.openaiKey is set inline — move it to a Kubernetes Secret and set litellm.existingSecret" -}}
    {{- end -}}
    {{- if and (ne (.Values.litellm.masterKey | default "") "") (eq (.Values.litellm.existingSecret | default "") "") -}}
      {{- $errs = append $errs "litellm.masterKey is set inline — move it to a Kubernetes Secret and set litellm.existingSecret" -}}
    {{- end -}}
  {{- end -}}

  {{- if .Values.cloudflareAI.enabled -}}
    {{- if and (ne (.Values.cloudflareAI.apiToken | default "") "") (eq (.Values.cloudflareAI.existingSecret | default "") "") -}}
      {{- $errs = append $errs "cloudflareAI.apiToken is set inline — move it to a Kubernetes Secret and set cloudflareAI.existingSecret" -}}
    {{- end -}}
  {{- end -}}

  {{- if .Values.browserless.enabled -}}
    {{- if and (ne (.Values.browserless.token | default "") "") (eq (.Values.browserless.existingSecret | default "") "") -}}
      {{- $errs = append $errs "browserless.token is set inline — move it to a Kubernetes Secret and set browserless.existingSecret" -}}
    {{- end -}}
  {{- end -}}

  {{- if .Values.minio.enabled -}}
    {{- if and (ne (.Values.minio.rootPassword | default "") "") (eq (.Values.minio.existingSecret | default "") "") -}}
      {{- $errs = append $errs "minio.rootPassword is set inline — move it to a Kubernetes Secret and set minio.existingSecret (leave blank to auto-generate)" -}}
    {{- end -}}
  {{- end -}}

  {{- if .Values.postgresql.enabled -}}
    {{- if and (ne (.Values.postgresql.auth.password | default "") "") (eq (.Values.postgresql.auth.existingSecret | default "") "") -}}
      {{- $errs = append $errs "postgresql.auth.password is set inline — move it to a Kubernetes Secret and set postgresql.auth.existingSecret (leave blank to auto-generate)" -}}
    {{- end -}}
    {{- if and (ne (.Values.postgresql.auth.postgresPassword | default "") "") (eq (.Values.postgresql.auth.existingSecret | default "") "") -}}
      {{- $errs = append $errs "postgresql.auth.postgresPassword is set inline — move it to a Kubernetes Secret and set postgresql.auth.existingSecret (leave blank to auto-generate)" -}}
    {{- end -}}
  {{- end -}}

  {{- if $errs -}}
    {{- fail (printf "\n\nNineVigil security posture violation: plaintext credentials detected in values.\n\n  %s\n\nSee docs/security/threat-model.md#secrets-handling.\nTo override (not recommended), set security.secrets.requireExistingSecret=false.\n" (join "\n  " $errs)) -}}
  {{- end -}}
{{- end -}}
{{- end -}}

{{/*
Assert: egress.strictMode must be enabled, and any allowlisted FQDN must be
explicit. When strictMode is disabled the chart fails render by default —
flip enforceOnRender to false to proceed (not recommended).
*/}}
{{- define "agentic-operator.assertEgressLocked" -}}
{{- if .Values.security.egress.enforceOnRender -}}
  {{- if not .Values.security.egress.strictMode -}}
    {{- fail "\n\nNineVigil security posture violation: security.egress.strictMode is false.\n\nAgent namespaces without default-deny egress re-introduce the Context.ai\nattack class. To override (not recommended), set\nsecurity.egress.enforceOnRender=false.\n" -}}
  {{- end -}}
  {{- range $i, $fqdn := .Values.security.egress.allowedFQDNs -}}
    {{- if or (not $fqdn) (eq $fqdn "*") -}}
      {{- fail (printf "security.egress.allowedFQDNs[%d]: wildcard or empty FQDNs are not permitted — list each destination explicitly" $i) -}}
    {{- end -}}
  {{- end -}}
{{- end -}}
{{- end -}}

{{/*
Assert: external OAuth is off unless operator explicitly opts in. Currently
no component performs OAuth against external SaaS, so the flag is a
forward-looking grep anchor for downstream forks.
*/}}
{{- define "agentic-operator.assertNoExternalOAuth" -}}
{{- if .Values.security.experimental.externalOAuth -}}
  {{- fail "\n\nNineVigil security posture violation: security.experimental.externalOAuth=true.\n\nThis flag is reserved for downstream forks that add third-party OAuth\nintegrations. The upstream chart ships no such component. If you set this\nto true in a fork, you are opting into the exact attack class described in\ndocs/security/threat-model.md — review before proceeding.\n" -}}
{{- end -}}
{{- end -}}

{{/*
Single entry-point evaluated by templates/security-assertions.yaml.
*/}}
{{- define "agentic-operator.runSecurityAssertions" -}}
{{- include "agentic-operator.assertNoPlaintextSecrets" . -}}
{{- include "agentic-operator.assertEgressLocked" . -}}
{{- include "agentic-operator.assertNoExternalOAuth" . -}}
{{- end -}}
