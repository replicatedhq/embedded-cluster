{{ define "troubleshoot.analyzers.analysis.logs" }}
- textAnalyze:
    checkName: Azure OpenAI Content Filter
    fileName: app/{{ include "analysis.name" . }}/logs/**/analysis.log
    regex: 'ResponsibleAIPolicyViolation'
    outcomes:
      - pass:
          when: "false"
          message: "No content filter policy violations detected."
      - fail:
          when: "true"
          message: |
            An analysis has triggered an [Azure OpenAI content filter](https://go.microsoft.com/fwlink/?linkid=2198766). You may need to [adjust the content filter](https://learn.microsoft.com/en-us/azure/ai-services/openai/how-to/content-filters) for your deployed models.
- textAnalyze:
    checkName: OpenAI Rate Limit
    fileName: app/{{ include "analysis.name" . }}/logs/**/analysis.log
    regex: 'openai.RateLimitError: Error code: 429'
    outcomes:
      - pass:
          when: "false"
          message: "No OpenAI rate limit errors detected."
      - fail:
          when: "true"
          message: "An analysis has triggered an OpenAI rate limit error. This may indicate that your OpenAI API key has reached its usage limits. Please review your OpenAI account to understand the limits and consider upgrading your plan if necessary."
{{- end }}