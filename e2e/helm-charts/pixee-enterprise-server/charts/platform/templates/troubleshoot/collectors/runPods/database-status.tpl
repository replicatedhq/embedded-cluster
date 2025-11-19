{{- define "troubleshoot.collectors.platform.runPods.database.status" -}}
- runPod:
    name: "collector-database-status"
    collectorName: "collector-database-status"
    namespace: "{{ .Release.Namespace }}"
    timeout: {{ max (.Values.global.pixee.runPodTimeout | regexReplaceAll "[^0-9]" "" | int) 300 }}s
    allowImagePullRetries: true
    podSpec:
      {{- with .Values.global.pixee.utility.image.pullSecrets }}
      imagePullSecrets:
        {{- toYaml . | nindent 8 }}
      {{- end }}

      containers:
        - name: collector-database-status
          image: {{ printf "%s/%s:%s" .Values.global.pixee.utility.image.registry .Values.global.pixee.utility.image.repository .Values.global.pixee.utility.image.tag | quote }}
          env:
            - name: DB_HOST
              {{- if .Values.database.embedded }}
              value: "{{ .Release.Name }}-{{ .Values.database.host }}"
              {{- else }}
              value: "{{ .Values.database.host }}"
              {{- end }}
            - name: DB_PORT
              value: "{{ .Values.database.port }}"
            - name: DB_NAME
              value: "{{ .Values.database.name }}"
            - name: DB_USER
              value: "{{ .Values.database.username }}"
            - name: DB_PASS
              {{- if .Values.database.existingSecret }}
              valueFrom:
                secretKeyRef:
                  name: {{ .Values.database.existingSecret }}
                  key: password
              {{- else }}
              value: "{{ .Values.database.password }}"
              {{- end }}
          command: ["/bin/sh"]
          args: 
            - -c
            - |
              #!/bin/sh
              set -eu

              # Main function
              main() {

                  echo "=========================================="
                  echo "    PIXEE DATABASE CONNECTIVITY TEST      "
                  echo "=========================================="
                  echo "Collection Time: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
                  echo ""

                  echo "Testing database connectivity..."
                  if ! pg_isready -d ${DB_NAME} -h ${DB_HOST} -p ${DB_PORT} -U ${DB_USER}; then
                      echo "ERROR: Database is not ready"
                      exit 1
                  fi
                  echo "Database connectivity test passed"

                  echo "Testing database access..."
                  if ! psql postgres://${DB_USER}:${DB_PASS}@${DB_HOST}:${DB_PORT}/${DB_NAME} -c "SELECT 1;" > /dev/null 2>&1; then
                      echo "ERROR: Unable to connect to database"
                      exit 1
                  fi
                  echo "Database access test passed"
              }

              # Run main function
              main
{{- end -}}