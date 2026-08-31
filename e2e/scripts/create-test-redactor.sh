#!/usr/bin/env bash
set -euox pipefail

main() {
    # Create a KOTS admin-console redactor spec that masks the known license ID.
    # This secret is read by the embedded-cluster support-bundle command via the
    # --redactors flag and proves that redactors are applied to CLI-generated bundles.
    kubectl apply -f - <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: kotsadm-redact-spec
  namespace: kotsadm
type: Opaque
stringData:
  redact-spec: |
    apiVersion: troubleshoot.sh/v1beta2
    kind: Redactor
    metadata:
      name: license-id-redactor
    spec:
      redactors:
        - name: License ID
          removals:
            regex:
              - redactor: "2cQCFfBxG7gXDmq1yAgPSM4OViF"
EOF
}

main "$@"
