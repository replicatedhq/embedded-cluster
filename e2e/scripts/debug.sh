#!/usr/bin/env bash
set -euox pipefail

main() {
  kubectl get pods -A
  kubectl describe pod kotsadm-0 -n kotsadm
  kubectl get services -A
  kubectl get installations -A
  kubectl get nodes -o yaml
  kubectl get events -A
  df -h
  free -mh
}

export KUBECONFIG=/var/lib/k0s/pki/admin.conf
export PATH=$PATH:/var/lib/embedded-cluster/bin
main "$@"
