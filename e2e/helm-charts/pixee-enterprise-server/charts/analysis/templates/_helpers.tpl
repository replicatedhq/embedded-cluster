{{/*
Expand the name of the chart.
*/}}
{{- define "analysis.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "analysis.fullname" -}}
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
Create chart name and version as used by the chart label.
*/}}
{{- define "analysis.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Sanitize app version for use in Kubernetes labels by removing digest and replacing invalid characters
*/}}
{{- define "analysis.sanitizedAppVersion" -}}
{{- if .Chart.AppVersion }}
{{- .Chart.AppVersion | regexReplaceAll "@.*$" "" | replace ":" "-" | trunc 63 }}
{{- end }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "analysis.labels" -}}
helm.sh/chart: {{ include "analysis.chart" . }}
{{ include "analysis.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ include "analysis.sanitizedAppVersion" . | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "analysis.selectorLabels" -}}
app.kubernetes.io/name: {{ include "analysis.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "analysis.serviceAccountName" -}}
{{- if .Values.serviceAccount.name }}
{{- .Values.serviceAccount.name }}
{{- else }}
{{- include "pixee-enterprise-server.serviceAccountName" . }}
{{- end }}
{{- end }}

