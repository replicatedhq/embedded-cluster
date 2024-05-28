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

export KUBECONFIG=/var/lib/k0s/pki/admin.conf
export PATH=$PATH:/var/lib/embedded-cluster/bin
main "$@"
