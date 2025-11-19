{{- define "troubleshoot.collectors.analysis.logs" }}
- logs:
    name: app/{{ include "analysis.name" . }}/logs
    selector:
      {{- $labels := (include "analysis.selectorLabels" . | fromYaml ) }}
      {{- range ( $labels | keys ) }}
      - {{ print . "=" (get $labels .) }}
      {{- end }}
    limits:
      maxAge: 720h
{{- end }}