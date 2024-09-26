#!/usr/bin/env bash
set -euox pipefail

DIR=/usr/local/bin
. $DIR/common.sh

main() {
  # create a nodeport service directly to kotsadm
  cat <<EOF | kubectl apply -f -
  apiVersion: v1
  kind: Service
  metadata:
    name: kotsadm-nodeport
    namespace: kotsadm
    labels:
      replicated.com/disaster-recovery: infra
      replicated.com/disaster-recovery-chart: admin-console
  spec:
    type: NodePort
    ports:
    - port: 30001
      targetPort: 3000
      nodePort: 30001
    selector:
      app: kotsadm
EOF
}

main "$@"
