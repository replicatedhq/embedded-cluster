{{- define "troubleshoot.collectors.platform.logs" }}
- logs:
    name: app/{{ include "platform.name" . }}/logs
    selector:
      {{- $labels := (include "platform.selectorLabels" . | fromYaml ) }}
      {{- range ( $labels | keys ) }}
      - {{ print . "=" (get $labels .) }}
      {{- end }}
    limits:
      maxAge: 720h
{{- end }}