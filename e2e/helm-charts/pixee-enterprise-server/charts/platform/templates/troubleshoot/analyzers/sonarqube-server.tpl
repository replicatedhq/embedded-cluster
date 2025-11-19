{{- define "troubleshoot.analyzers.platform.sonarqube-server" }}
- textAnalyze:
    checkName: "Validate SonarQube Server Token"
    fileName: "static/data.txt"
    regex: "SUCCESS: Sonar token starts with 'squ_'"
    outcomes:
      - pass:
          when: "true"
          message: "SonarQube Server token is valid."
      - fail:
          when: "false"
          message: "SonarQube Server user token is invalid. Please verify your SonarQube Server token in the configuration."
{{- end }}