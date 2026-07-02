{{- define "agentic-operator-sub.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := "agentic-operator" }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{- define "agentic-operator-sub.labels" -}}
app.kubernetes.io/name: agentic-operator
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/component: operator
app.kubernetes.io/part-of: agentic-operator
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version }}
{{- end }}

{{- define "agentic-operator-sub.selectorLabels" -}}
app.kubernetes.io/name: agentic-operator
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{- define "agentic-operator-sub.serviceAccountName" -}}
{{- printf "%s-operator" (include "agentic-operator-sub.fullname" .) }}
{{- end }}

{{- define "agentic-operator-sub.licenseName" -}}
{{- printf "%s-license" (include "agentic-operator-sub.fullname" .) }}
{{- end }}
