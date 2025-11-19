{{- define "troubleshoot.collectors.pixee-enterprise-server.azure-openai" -}}
{{- $models := keys .Values.global.pixee.ai.azure.deployments | sortAlpha }}
{{- range $index, $model := $models }}
- http:
    collectorName: "az-openai-model-deployment-{{ $index }}"
    post:
      url: "{{ $.Values.global.pixee.ai.azure.endpoint }}/openai/deployments/{{ index $.Values.global.pixee.ai.azure.deployments $model }}/chat/completions?api-version=2024-12-01-preview"
      headers:
        api-key: {{ $.Values.global.pixee.ai.azure.key | quote }}
        content-type: "application/json"
      body: |
        {
          "messages": [{"role": "user", "content": "Hello"}]
        }
    saveResponse: true
{{- end }}
{{- end -}}