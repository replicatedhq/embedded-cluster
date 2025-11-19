{{- define "troubleshoot.collectors.pixee-enterprise-server.openai" -}}
{{- $models := list "gpt-4o" "o3-mini" }}
{{- $baseUrl := .Values.global.pixee.ai.openai.baseUrl | default "https://api.openai.com/v1" }}
{{- $baseUrl = $baseUrl | trimSuffix "/" }}
{{- $endpoint := printf "%s/chat/completions" $baseUrl }}
{{- range $index, $model := $models }}
- http:
    collectorName: "openai-model-access-{{ $index }}"
    post:
      url: "{{ $endpoint }}"
      headers:
        authorization: "Bearer {{ $.Values.global.pixee.ai.openai.key }}"
        content-type: "application/json"
      body: |
        {
          "model": "{{ $model }}",
          "messages": [{"role": "user", "content": "Hello"}]
        }
{{- end }}
{{- end -}}