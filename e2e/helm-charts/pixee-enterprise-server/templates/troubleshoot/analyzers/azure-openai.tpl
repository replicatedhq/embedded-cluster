{{- define "troubleshoot.analyzers.pixee-enterprise-server.azure-openai" -}}
{{- $models := keys .Values.global.pixee.ai.azure.deployments | sortAlpha }}
{{- range $index, $model := $models }}
- http:
    checkName: "Azure OpenAI model deployment for `{{ $model }}` must be accessible."
    collectorName: "az-openai-model-deployment-{{ $index }}"
    outcomes:
      - warn:
          when: "error"
          message: |
            An error occurred when attempting to validate your Azure OpenAI model `{{ $model }}`
            deployment named `{{ index $.Values.global.pixee.ai.azure.deployments $model }}`, please try again.
      - pass:
          when: "statusCode == 200"
          message: |
            Azure OpenAI model `{{ $model }}` deployment named
            `{{ index $.Values.global.pixee.ai.azure.deployments $model }}` is accessible.
      - fail:
          when: "statusCode == 429"
          message: |
            Azure OpenAI model `{{ $model }}` deployment named
            `{{ index $.Values.global.pixee.ai.azure.deployments $model }}` is being rate limited.
            Please check your Azure OpenAI usage and limits.
      - fail:
          message: |
            Azure OpenAI model `{{ $model }}` deployment named `{{ index $.Values.global.pixee.ai.azure.deployments $model }}`
            is not accessible or does not exist. Double check your Pixee
            Enterprise Server configuration and your [Azure](https://portal.azure.com) resources.
{{- end }}
{{- end -}}