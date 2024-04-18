#!/usr/bin/env bash
set -euox pipefail

main() {
  kubectl get pods -A
  kubectl get services -A
  kubectl get installations -A
  kubectl get nodes -A
}

export KUBECONFIG=/var/lib/k0s/pki/admin.conf
export PATH=$PATH:/var/lib/embedded-cluster/bin
main "$@"
