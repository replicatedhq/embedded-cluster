{{- define "troubleshoot.analyzers.platform.database.embedded" }}
- statefulsetStatus:
    name: {{ .Release.Name }}-postgresql
    namespace: {{ .Release.Namespace }}
    outcomes:
      - fail:
          when: "absent" # note that the "absent" failure state must be listed first if used.
          message: The Pixee Enterprise Server embedded database (postgresql) is not present.
      - fail:
          when: "< 1"
          message: The Pixee Enterprise Server embedded database (postgresql) is not ready.
      - pass:
          message: The Pixee Enterprise Server embedded database (postgresql) is ready.
{{- end }}
{{- define "troubleshoot.analyzers.platform.database.status" }}
- textAnalyze:
    checkName: Verify Database Connectivity
    fileName: collector-database-status/collector-database-status.log
    regex: "ERROR: Database is not ready"
    outcomes:
        - pass:
            when: "false"
            message: "Database is reachable"
        - fail:
            when: "true"
            message: "Problem reaching the specified host/port"
- textAnalyze:
    checkName: Verify Database Credentials
    fileName: collector-database-status/collector-database-status.log
    regex: "ERROR: Unable to connect to database"
    outcomes:
        - pass:
            when: "false"
            message: "Connection established to the database"
        - fail:
            when: "true"
            message: "Problem with the username/password"
{{- end }}