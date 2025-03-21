#!/usr/bin/env bash
set -euox pipefail

DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
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
      kots.io/kotsadm: "true"
  spec:
    type: NodePort
    ports:
    - port: 30003
      targetPort: 3000
      nodePort: 30003
    selector:
      app: kotsadm
EOF
}

main "$@"
