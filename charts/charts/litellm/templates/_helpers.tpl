{{- define "litellm.fullname" -}}
{{- printf "%s-litellm" .Release.Name | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "litellm.labels" -}}
app.kubernetes.io/name: litellm
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/component: llm-proxy
app.kubernetes.io/part-of: agentic-operator
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version }}
{{- end }}

{{- define "litellm.selectorLabels" -}}
app.kubernetes.io/name: litellm
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Resolve the Secret name that holds OPENAI_API_KEY / LITELLM_MASTER_KEY.
If .Values.existingSecret is set, use it (BYO secret -- recommended for production).
Otherwise fall back to the chart-managed Secret (dev/demo only).
*/}}
{{- define "litellm.secretName" -}}
{{- if .Values.existingSecret -}}
{{ .Values.existingSecret }}
{{- else -}}
{{ include "litellm.fullname" . }}-secrets
{{- end -}}
{{- end }}
