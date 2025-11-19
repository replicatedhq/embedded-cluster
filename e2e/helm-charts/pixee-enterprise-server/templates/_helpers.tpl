{{/*
Expand the name of the chart.
*/}}
{{- define "pixee-enterprise-server.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "pixee-enterprise-server.fullname" -}}
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
{{- define "pixee-enterprise-server.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Sanitize app version for use in Kubernetes labels by removing digest and replacing invalid characters
*/}}
{{- define "pixee-enterprise-server.sanitizedAppVersion" -}}
{{- if .Chart.AppVersion }}
{{- .Chart.AppVersion | regexReplaceAll "@.*$" "" | replace ":" "-" | trunc 63 }}
{{- end }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "pixee-enterprise-server.labels" -}}
helm.sh/chart: {{ include "pixee-enterprise-server.chart" . }}
{{ include "pixee-enterprise-server.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ include "pixee-enterprise-server.sanitizedAppVersion" . | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "pixee-enterprise-server.selectorLabels" -}}
app.kubernetes.io/name: {{ include "pixee-enterprise-server.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "pixee-enterprise-server.serviceAccountName" -}}
{{- if and .Values.global.pixee.serviceAccount.create .Values.global.pixee.serviceAccount.name }}
{{- .Values.global.pixee.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.global.pixee.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Render security context with intelligent property merging
Usage: {{- include "pixee-enterprise-server.renderSecurityContext" .Values.securityContext | nindent 12 }}

This helper automatically omits empty seLinuxOptions to avoid Kubernetes validation issues.
Some Kubernetes versions/distributions reject empty seLinuxOptions: {} objects, and 
admission controllers may flag them as invalid. When seLinuxOptions has actual values,
they are rendered correctly. This follows Bitnami's pattern for maximum compatibility.
*/}}
{{/*
Check if security context has any actual values to render after cleanup
*/}}
{{- define "pixee-enterprise-server.hasSecurityContext" -}}
{{- $context := . -}}
{{/* Remove empty seLinuxOptions to avoid validation issues with admission controllers */}}
{{- if and (hasKey $context "seLinuxOptions") (not $context.seLinuxOptions) -}}
  {{- $context = omit $context "seLinuxOptions" -}}
{{- end -}}
{{/* Return true if context has actual properties after cleanup */}}
{{- if $context -}}
true
{{- end -}}
{{- end }}

{{- define "pixee-enterprise-server.renderSecurityContext" -}}
{{- $context := . -}}
{{/* Remove empty seLinuxOptions to avoid validation issues with admission controllers */}}
{{- if and (hasKey $context "seLinuxOptions") (not $context.seLinuxOptions) -}}
  {{- $context = omit $context "seLinuxOptions" -}}
{{- end -}}
{{- $context | toYaml -}}
{{- end }}

{{/*
Create environment from replicated customer name
*/}}
{{- define "pixee-enterprise-server.environment" -}}
{{- printf "pixee-enterprise-server-%s" (dig "replicated" "customerName" "development" .Values.global | lower | replace " " "-") }}
{{- end }}

{{/*
Pixee Enterprise Server secrets checksum
*/}}
{{- define "pixee-enterprise-server.secretsChecksum" -}}
{{- include (print "pixee-enterprise-server/templates/secrets.yaml") . | sha256sum }}
{{- end }}

{{- define "pixee-enterprise-server.proxy.enabled" -}}
{{- if or (.Values.global.pixee.httpProxy) (.Values.global.pixee.httpsProxy) -}}true{{- else -}}false{{- end -}}
{{- end -}}

{{- define "pixee-enterprise-server.privateCACert" -}}
{{- if .Values.global.pixee.privateCACert -}}
{{- $configMap := lookup "v1" "ConfigMap" .Release.Namespace .Values.global.pixee.privateCACert -}}
{{- if $configMap -}}
{{- $hasContent := false -}}
{{- range $key, $value := $configMap.data -}}
{{- if and $value (ne (trim $value) "") -}}
{{- $hasContent = true -}}
{{- end -}}
{{- end -}}
{{- $hasContent -}}
{{- else -}}
false
{{- end -}}
{{- else -}}
false
{{- end -}}
{{- end -}}

{{/*
URL Pattern Matcher Helper Template
Usage: {{ include "pixee-enterprise-server.urlPatternMatch" (dict "patterns" "*.example.com,api.*.com,localhost:*" "url" "api.test.com") }}
Returns: "true" if URL matches any pattern, "false" otherwise

Parameters:
- patterns: comma-separated string of URL patterns (supports * wildcards)
- url: the URL string to test against the patterns
*/}}
{{- define "pixee-enterprise-server.urlPatternMatch" -}}
{{- $patterns := .patterns -}}
{{- $url := .url -}}
{{- $match := false -}}

{{- if and $patterns $url -}}
  {{- range $pattern := splitList "," $patterns -}}
    {{- $pattern = trim $pattern -}}
    {{- if not $match -}}
      {{- if eq $pattern "*" -}}
        {{- $match = true -}}
      {{- else if contains "*" $pattern -}}
        {{- /* Handle wildcard patterns */ -}}
        {{- $escapedPattern := $pattern -}}
        {{- /* Escape special regex characters except * */ -}}
        {{- $escapedPattern = regexReplaceAll "\\." $escapedPattern "\\." -}}
        {{- $escapedPattern = regexReplaceAll "\\+" $escapedPattern "\\+" -}}
        {{- $escapedPattern = regexReplaceAll "\\?" $escapedPattern "\\?" -}}
        {{- $escapedPattern = regexReplaceAll "\\^" $escapedPattern "\\^" -}}
        {{- $escapedPattern = regexReplaceAll "\\$" $escapedPattern "\\$" -}}
        {{- $escapedPattern = regexReplaceAll "\\|" $escapedPattern "\\|" -}}
        {{- $escapedPattern = regexReplaceAll "\\(" $escapedPattern "\\(" -}}
        {{- $escapedPattern = regexReplaceAll "\\)" $escapedPattern "\\)" -}}
        {{- $escapedPattern = regexReplaceAll "\\[" $escapedPattern "\\[" -}}
        {{- $escapedPattern = regexReplaceAll "\\]" $escapedPattern "\\]" -}}
        {{- $escapedPattern = regexReplaceAll "\\{" $escapedPattern "\\{" -}}
        {{- $escapedPattern = regexReplaceAll "\\}" $escapedPattern "\\}" -}}
        {{- /* Convert * to .* for regex */ -}}
        {{- $regexPattern := regexReplaceAll "\\*" $escapedPattern ".*" -}}
        {{- /* Add anchors for full match */ -}}
        {{- $regexPattern = printf "^%s$" $regexPattern -}}
        {{- if regexMatch $regexPattern $url -}}
          {{- $match = true -}}
        {{- end -}}
      {{- else -}}
        {{- /* Exact match */ -}}
        {{- if eq $pattern $url -}}
          {{- $match = true -}}
        {{- end -}}
      {{- end -}}
    {{- end -}}
  {{- end -}}
{{- end -}}

{{- if $match -}}
true
{{- else -}}
false
{{- end -}}
{{- end -}}
