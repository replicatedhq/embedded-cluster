{{- define "troubleshoot.analyzers.platform.cluster" }}
- deploymentStatus:
    name: {{ include "platform.fullname" . }}
    namespace: {{ .Release.Namespace }}
    outcomes:
      - fail:
          when: "absent" # note that the "absent" failure state must be listed first if used.
          message: The platform service deployment is not present.
      - fail:
          when: "< 1"
          message: The platform service deployment does not have any ready replicas.
      - pass:
          message: The platform service deployment is ready.
{{- end }}