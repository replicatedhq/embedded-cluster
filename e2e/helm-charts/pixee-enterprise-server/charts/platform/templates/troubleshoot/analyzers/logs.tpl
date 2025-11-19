{{ define "troubleshoot.analyzers.platform.logs" }}
- textAnalyze:
    checkName: Hostname configuration
    fileName: app/{{ include "platform.name" . }}/logs/**/pixeebot.log
    regex: 'The config property pixee.pixeebot.analysis-input.allowed-audience is required but it could not be found in any config source'
    outcomes:
      - pass:
          when: "false"
          message: "The url for the server is configured correctly"
      - fail:
          when: "true"
          message: |
            The hostname for your instance is configured incorrectly. Select "Config" in the
            Admin Console and change the value for "Hostname".
{{- end }}