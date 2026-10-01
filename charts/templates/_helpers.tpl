{{/*
Expand the name of the chart.
*/}}
{{- define "agentic-operator.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this
(by the DNS naming spec). If release name contains chart name it will be used
as a full name.
*/}}
{{- define "agentic-operator.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart label.
*/}}
{{- define "agentic-operator.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels applied to every resource.
*/}}
{{- define "agentic-operator.labels" -}}
helm.sh/chart: {{ include "agentic-operator.chart" . }}
{{ include "agentic-operator.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: agentic-operator
{{- end }}

{{/*
Selector labels — stable set used in matchLabels.
*/}}
{{- define "agentic-operator.selectorLabels" -}}
app.kubernetes.io/name: {{ include "agentic-operator.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Target namespace — prefers .Values.global.namespace, falls back to release namespace.
*/}}
{{- define "agentic-operator.namespace" -}}
{{- default .Release.Namespace .Values.global.namespace }}
{{- end }}

{{/*
Create the name of the service account to use.
*/}}
{{- define "agentic-operator.serviceAccountName" -}}
{{- printf "%s-operator" (include "agentic-operator.fullname" .) }}
{{- end }}

{{/*
Image pull secrets list.
*/}}
{{- define "agentic-operator.imagePullSecrets" -}}
{{- with .Values.global.imagePullSecrets }}
imagePullSecrets:
{{- range . }}
  - name: {{ . }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Render a full image reference: [registry/]repo:tag
*/}}
{{- define "agentic-operator.image" -}}
{{- $registry := .global.imageRegistry -}}
{{- $repo := .image.repository -}}
{{- $tag := .image.tag | default "latest" -}}
{{- if $registry -}}
{{- printf "%s/%s:%s" $registry $repo $tag -}}
{{- else -}}
{{- printf "%s:%s" $repo $tag -}}
{{- end -}}
{{- end }}

{{/*
receipt-writer labels. Own app name so operator selectors never match it.
*/}}
{{- define "agentic-operator.receiptWriter.selectorLabels" -}}
app.kubernetes.io/name: receipt-writer
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{- define "agentic-operator.receiptWriter.labels" -}}
helm.sh/chart: {{ include "agentic-operator.chart" . }}
{{ include "agentic-operator.receiptWriter.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: agentic-operator
app.kubernetes.io/component: receipt-writer
{{- end }}
