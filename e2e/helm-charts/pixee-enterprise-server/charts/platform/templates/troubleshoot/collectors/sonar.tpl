{{- define "troubleshoot.collectors.platform.sonar" }}
{{ $sonarBaseUri := .Values.sonar.baseUri }}
{{ $sonarHostname := (urlParse $sonarBaseUri).hostname }}
{{ $noProxy := include "pixee-enterprise-server.urlPatternMatch" (dict "patterns" .Values.global.pixee.noProxy "url" $sonarHostname)}}
- http:
    collectorName: "sonar-issues-search-test"
    get:
      url: "{{ $sonarBaseUri }}/api/issues/search?componentKeys=test-data"
      {{- if and (ne .Values.global.pixee.httpsProxy "") (eq $noProxy "false") }}
      proxy: "{{ .Values.global.pixee.httpsProxy }}"
      {{- else if and (ne .Values.global.pixee.httpProxy "") (eq $noProxy "false") }}
      proxy: "{{ .Values.global.pixee.httpProxy }}"
      {{- end }}
      headers:
        Authorization: "Bearer {{ .Values.sonar.token }}"
        Accept: "application/json"
    saveResponse: true

- http:
    collectorName: "sonar-hotspots-search-test"
    get:
      url: "{{ $sonarBaseUri }}/api/hotspots/search?hotspots=test-data"
      {{- if and (ne .Values.global.pixee.httpsProxy "") (eq $noProxy "false") }}
      proxy: "{{ .Values.global.pixee.httpsProxy }}"
      {{- else if and (ne .Values.global.pixee.httpProxy "") (eq $noProxy "false") }}
      proxy: "{{ .Values.global.pixee.httpProxy }}"
      {{- end }}
      headers:
        Authorization: "Bearer {{ .Values.sonar.token }}"
        Accept: "application/json"
    saveResponse: true

{{- end -}}