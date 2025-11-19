{{- define "troubleshoot.collectors.platform.sonarqube-server" }}
- data:
    name: static/data.txt
    data: |
      {{- if not .Values.sonar.token }}
      ERROR: Sonar token is empty
      {{- else if regexMatch "^squ_" .Values.sonar.token }}
      SUCCESS: Sonar token starts with 'squ_' (length: {{ len .Values.sonar.token }})
      {{- else }}
      ERROR: Sonar token does not start with 'squ_' (starts with: {{ substr 0 4 .Values.sonar.token }}...)
      {{- end }}
{{- end }}