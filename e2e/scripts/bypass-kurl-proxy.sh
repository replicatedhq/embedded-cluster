#!/usr/bin/env bash
set -euox pipefail

main() {
  kubectl expose pod kotsadm-0 -n kotsadm --type=NodePort --port=30001 --target-port=3000 --name=kotsadm-nodeport
  kubectl patch svc kotsadm-nodeport -n kotsadm --type='json' -p '[{"op":"replace","path":"/spec/ports/0/nodePort","value":30001}]'
}

export KUBECONFIG=/var/lib/k0s/pki/admin.conf
export PATH=$PATH:/var/lib/embedded-cluster/bin
main "$@"
