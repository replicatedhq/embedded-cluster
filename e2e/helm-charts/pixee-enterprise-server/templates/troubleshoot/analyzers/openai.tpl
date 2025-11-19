{{- define "troubleshoot.analyzers.pixee-enterprise-server.openai" -}}
{{- $models := list "gpt-4o" "o3-mini" }}
{{- range $index, $model := $models }}
- http:
    checkName: "OpenAI API must be accessible with valid credentials to access the {{ $model }} model."
    collectorName: "openai-model-access-{{ $index }}"
    outcomes:
      - pass:
          when: "statusCode == 200"
          message: |
            OpenAI API is accessible and your credentials are valid.
      - warn:
          when: "error"
          message: |
            An error occurred when attempting to validate your OpenAI API credentials, please check your network settings and try again.
      - fail:
          when: "statusCode == 401"
          message: |
            OpenAI API credentials are invalid, please check your Pixee Enterprise Server configuration.
      - fail:
          when: "statusCode == 403"
          message: |
            OpenAI API credentials are invalid, please check your Pixee Enterprise Server configuration.
      - fail:
            when: "statusCode == 429"
            message: |
                OpenAI API rate limit has been reached, please check your OpenAI API usage/limits.
      - fail:
          message: |
            OpenAI API credentials could not be validated, please check your Pixee Enterprise Server configuration.
{{- end }}
{{- end -}}
