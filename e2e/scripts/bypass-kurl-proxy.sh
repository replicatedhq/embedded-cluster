#!/usr/bin/env bash
set -euox pipefail

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
    - port: 30003
      targetPort: 3000
      nodePort: 30003
    selector:
      app: kotsadm
EOF
}

export KUBECONFIG=/var/lib/k0s/pki/admin.conf
export PATH=$PATH:/var/lib/embedded-cluster/bin
main "$@"
