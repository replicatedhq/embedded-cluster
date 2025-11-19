{{ define "troubleshoot.analyzers.analysis.cluster" }}
- deploymentStatus:
    name: {{ include "analysis.fullname" . }}
    namespace: {{ .Release.Namespace }}
    outcomes:
      - fail:
          when: "absent" # note that the "absent" failure state must be listed first if used.
          message: The analysis service deployment is not present.
      - fail:
          when: "< 1"
          message: The analysis service deployment does not have any ready replicas.
      - pass:
          message: The analysis service deployment is ready.
{{- end }}