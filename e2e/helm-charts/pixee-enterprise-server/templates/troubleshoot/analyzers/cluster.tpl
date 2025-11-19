{{- define "troubleshoot.analyzers.pixee-enterprise-server.cluster" -}}
- clusterVersion:
    checkName: Is this cluster running a supported Kubernetes version
    outcomes:
      - fail:
          when: "< 1.26.0"
          message: |
            Your current version of Kubernetes is current unsupported by the
            Kubernetes community. If you have extended support available from
            your Kubernetes vendor you can ignore this error.
          uri: https://www.kubernetes.io
      - warn:
          when: "< 1.28.0"
          message: |
            Your current version of Kubernetes is supported by the community
            but it is not the latest version. Review the [support
            period](https://kubernetes.io/releases/patch-releases/#support-period)
            for Kubernetes release to understand when your version will go out
            of support.
          uri: https://kubernetes.io
      - pass:
          message: |
            You are running the current version of Kubernetes.
- nodeResources:
    checkName: The cluster should have at least 32 GB of memory.
    outcomes:
    - warn:
        # Allowing some leeway for conversion issues.
        when: "sum(memoryCapacity) <= 30Gi"
        message: "The cluster should have at least 32 GB of memory."
    - pass:
        message: "The cluster has at least 32 GB of memory."
- nodeResources:
    checkName: The cluster should have at least 8 cores.
    outcomes:
      - warn:
          when: "min(cpuCapacity) <= 7"
          message: "The cluster should have at least 8 cores."
      - pass:
          message: "The cluster has at least 8 cores."
- nodeResources:
    checkName: The cluster should have at least 100 GB of ephemeral storage.
    outcomes:
      - warn:
          when: "sum(ephemeralStorageCapacity) <= 100Gi"
          message: "The cluster should have at least 100 GB of storage."
      - pass:
          message: "The cluster has at least 100 GB of storage."
{{- end -}}
{{- define "troubleshoot.analyzers.pixee-enterprise-server.cluster.minio" -}}
- deploymentStatus:
    name: {{ include "pixee-enterprise-server.fullname" . }}-minio
    namespace: {{ .Release.Namespace }}
    outcomes:
      - fail:
          when: "absent" # note that the "absent" failure state must be listed first if used.
          message: The Minio deployment is not present.
      - fail:
          when: "< 1"
          message: The Minio deployment does not have any ready replicas.
      - pass:
          message: There are multiple replicas of the Minio deployment ready.
{{- end -}}