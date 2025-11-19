{{- define "troubleshoot.analyzers.platform.sonar" }}
- http:
    checkName: Verify Sonar permission issues search at {{ .Values.sonar.baseUri }}
    collectorName: "sonar-issues-search-test"
    outcomes:
      - warn:
          when: "error"
          message: |
            An error occurred when attempting to validate your Sonar permissions with issues search, please try again.
      - pass:
          when: "statusCode == 200"
          message: |
            Sonar Issues Search permissions valid.
      - fail:
          when: "statusCode == 401"
          message: |
            Invalid Sonar credentials, please check your Sonar credentials and try again.
      - fail:
          message: |
            Error connecting to Sonar, please check your Pixee Enterprise Server configuration.
- http:
    checkName: Verify Sonar permission hotspots search at {{ .Values.sonar.baseUri }}
    collectorName: "sonar-hotspots-search-test"
    outcomes:
      - warn:
          when: "error"
          message: |
            An error occurred when attempting to validate your Sonar permissions with Hotspots Search, please try again.
      - pass:
          when: "statusCode == 200"
          message: |
            Sonar Hotspots Search permissions valid.
      - fail:
          when: "statusCode == 401"
          message: |
            Invalid Sonar credentials, please check your Sonar credentials and try again.
      - fail:
          message: |
            Error connecting to Sonar, please check your Pixee Enterprise Server configuration.
{{- end }}