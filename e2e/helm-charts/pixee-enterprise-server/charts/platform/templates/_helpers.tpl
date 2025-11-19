{{/*
Expand the name of the chart.
*/}}
{{- define "platform.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "platform.fullname" -}}
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
{{- define "platform.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Sanitize app version for use in Kubernetes labels by removing digest and replacing invalid characters
*/}}
{{- define "platform.sanitizedAppVersion" -}}
{{- if .Chart.AppVersion }}
{{- .Chart.AppVersion | regexReplaceAll "@.*$" "" | replace ":" "-" | trunc 63 }}
{{- end }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "platform.labels" -}}
helm.sh/chart: {{ include "platform.chart" . }}
{{ include "platform.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ include "platform.sanitizedAppVersion" . | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "platform.selectorLabels" -}}
app.kubernetes.io/name: {{ include "platform.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "platform.serviceAccountName" -}}
{{- if .Values.serviceAccount.name }}
{{- .Values.serviceAccount.name }}
{{- else }}
{{- include "pixee-enterprise-server.serviceAccountName" . }}
{{- end }}
{{- end }}

{{/*
Build the platform service url based on selected protocol and domain
*/}}
{{- define "platform.url" -}}
{{- printf "%s://%s" .Values.global.pixee.protocol .Values.global.pixee.domain }}
{{- end -}}

{{/*
Build the database host name based on database type
*/}}
{{- define "platform.database.host" -}}
{{- if .database.embedded -}}
{{- printf "%s-%s" .Release.Name .database.host -}}
{{- else -}}
{{- .database.host -}}
{{- end -}}
{{- end -}}

{{/*
Database connection string for JDBC
*/}}
{{- define "platform.database.connection.jdbc" -}}
jdbc:postgresql://{{ include "platform.database.host" . }}:{{ .database.port }}/{{ .database.name }}
{{- end -}}

{{- define "platform.proxy.javaToolOptions" -}}
{{- $opts := list -}}
{{- if .Values.global.pixee.httpProxy -}}
{{- $parsed := urlParse (.Values.global.pixee.httpProxy) -}}
{{- $opts = append $opts (printf "-Dhttp.proxyHost=%s" $parsed.hostname) -}}
{{- if $parsed.port -}}
{{- $opts = append $opts (printf "-Dhttp.proxyPort=%s" $parsed.port) -}}
{{- else -}}
{{- /* Fallback: extract port manually */ -}}
{{- $hostPort := $parsed.host -}}
{{- if contains ":" $hostPort -}}
{{- $portStr := last (splitList ":" $hostPort) -}}
{{- $opts = append $opts (printf "-Dhttp.proxyPort=%s" $portStr) -}}
{{- end -}}
{{- end -}}
{{- end -}}
{{- if .Values.global.pixee.httpsProxy -}}
{{- $parsed := urlParse (.Values.global.pixee.httpsProxy) -}}
{{- $opts = append $opts (printf "-Dhttps.proxyHost=%s" $parsed.hostname) -}}
{{- if $parsed.port -}}
{{- $opts = append $opts (printf "-Dhttps.proxyPort=%s" $parsed.port) -}}
{{- else -}}
{{- /* Fallback: extract port manually */ -}}
{{- $hostPort := $parsed.host -}}
{{- if contains ":" $hostPort -}}
{{- $portStr := last (splitList ":" $hostPort) -}}
{{- $opts = append $opts (printf "-Dhttps.proxyPort=%s" $portStr) -}}
{{- end -}}
{{- end -}}
{{- end -}}
{{- $noProxyHosts := list -}}
{{- if .Values.global.pixee.noProxy -}}
{{- $noProxyHosts = concat $noProxyHosts (splitList "," (.Values.global.pixee.noProxy)) -}}
{{- end -}}
{{- /* Add internal Kubernetes services that should bypass proxy */ -}}
{{- $noProxyHosts = append $noProxyHosts "*.local" -}}
{{- $noProxyHosts = append $noProxyHosts "*.svc.cluster.local" -}}
{{- $noProxyHosts = append $noProxyHosts (printf "%s-*" .Release.Name) -}}
{{- $noProxyHosts = append $noProxyHosts "replicated" -}}
{{- if $noProxyHosts -}}
{{- $opts = append $opts (printf "-Dhttp.nonProxyHosts=%s" (join "|" ($noProxyHosts | uniq))) -}}
{{- end -}}
{{- join " " $opts -}}
{{- end -}}

{{/*
Trust store Java tool options for configuring SSL trust store
*/}}
{{- define "platform.trustStore.javaToolOptions" -}}
{{- $opts := list -}}
{{- if include "pixee-enterprise-server.privateCACert" . -}}
{{- $opts = append $opts "-Djavax.net.ssl.trustStore=/var/ssl/enterprise-truststore.p12" -}}
{{- $opts = append $opts "-Djavax.net.ssl.trustStoreType=PKCS12" -}}
{{- $opts = append $opts "-Djavax.net.ssl.trustStorePassword=changeit" -}}
{{- end -}}
{{- join " " $opts -}}
{{- end -}}

{{/*
Combined Java tool options for proxy and trust store configuration
*/}}
{{- define "platform.javaToolOptions" -}}
{{- $proxyOpts := include "platform.proxy.javaToolOptions" . -}}
{{- $trustStoreOpts := include "platform.trustStore.javaToolOptions" . -}}
{{- $allOpts := list -}}
{{- if $proxyOpts -}}
{{- $allOpts = append $allOpts $proxyOpts -}}
{{- end -}}
{{- if $trustStoreOpts -}}
{{- $allOpts = append $allOpts $trustStoreOpts -}}
{{- end -}}
{{- join " " $allOpts -}}
{{- end -}}