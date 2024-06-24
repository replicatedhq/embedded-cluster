#!/usr/bin/env bash
set -euox pipefail

main() {
    sleep 10 # wait for kubectl to become available

    echo "pods"
    kubectl get pods -A -o wide
    echo "services"
    kubectl get services -A
}

export EMBEDDED_CLUSTER_METRICS_BASEURL="https://staging.replicated.app"
export KUBECONFIG=/var/lib/k0s/pki/admin.conf
export PATH=$PATH:/var/lib/embedded-cluster/bin
main "$@"
