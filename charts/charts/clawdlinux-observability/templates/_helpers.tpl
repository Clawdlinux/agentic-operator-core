{{/*
clawdlinux-observability — Helm template helpers
*/}}

{{- define "clawd-observability.fullname" -}}
{{- printf "%s-%s" .Release.Name .Chart.Name | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "clawd-observability.namespace" -}}
{{- default .Release.Namespace .Values.global.namespace -}}
{{- end -}}

{{- define "clawd-observability.labels" -}}
app.kubernetes.io/name: {{ .Chart.Name }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version }}
clawd.component: observability
{{- end -}}

{{- define "clawd-observability.componentLabels" -}}
{{ include "clawd-observability.labels" . }}
app.kubernetes.io/component: {{ .component }}
{{- end -}}

{{- define "clawd-observability.image" -}}
{{- $img := .image -}}
{{- $reg := .root.Values.global.imageRegistry | default "" -}}
{{- if $reg -}}
{{- printf "%s/%s:%s" $reg $img.repository $img.tag -}}
{{- else -}}
{{- printf "%s:%s" $img.repository $img.tag -}}
{{- end -}}
{{- end -}}

{{- define "clawd-observability.clickhouse.host" -}}
{{- printf "%s-clickhouse" (include "clawd-observability.fullname" .) -}}
{{- end -}}

{{- define "clawd-observability.tempo.endpoint" -}}
{{- printf "%s-tempo:4317" (include "clawd-observability.fullname" .) -}}
{{- end -}}

{{- define "clawd-observability.collector.endpoint" -}}
{{- printf "%s-otel-collector:4317" (include "clawd-observability.fullname" .) -}}
{{- end -}}
