{{- define "troubleshoot.collectors.platform.object-store" -}}
{{ if .Values.global.pixee.objectStore.embedded }}
- exec:
    args:
    - -rf
    - /tmp/minio-archive.tar.gz
    - /tmp/minio/local/pixee-analysis-service
    - /tmp/minio/local/pixee-analysis-input
    collectorName: cleanup-minio
    command:
    - rm
    namespace: {{ .Release.Namespace }}
    selector:
    - app.kubernetes.io/name=minio
    timeout: 30s

- exec:
    args:
    - find
    - local/pixee-analysis-service
    - --newer-than
    - 24h
    - --exec
    - 'mc cp -a --recursive {} /tmp/minio/{}'
    collectorName: minio-extraction-service
    command:
    - mc
    namespace: {{ .Release.Namespace }}
    selector:
    - app.kubernetes.io/name=minio
    timeout: 30s

- exec:
    args:
    - find
    - local/pixee-analysis-input
    - -path
    - "*/tools/*"
    - --ignore
    - "*.zip"
    - --newer-than
    - 24h
    - --exec
    - 'mc cp -a {} /tmp/minio/{}'
    collectorName: minio-extraction-input
    command:
    - mc
    namespace: {{ .Release.Namespace }}
    selector:
    - app.kubernetes.io/name=minio
    timeout: 30s

- exec:
    args:
    - -czf
    - /tmp/minio-archive.tar.gz
    - -C
    - /tmp/minio
    - .
    collectorName: compress-minio
    command:
    - tar
    namespace: {{ .Release.Namespace }}
    selector:
    - app.kubernetes.io/name=minio
    timeout: 30s

- copy:
    collectorName: copy-minio
    containerName: minio
    containerPath: /tmp/minio-archive.tar.gz
    namespace: {{ .Release.Namespace }}
    selector:
    - app.kubernetes.io/name=minio
{{- end }}
{{- end -}}