#!/usr/bin/env bash
set -euox pipefail

main() {
    local pod_cidr="$1"
    local service_cidr="$2"
    sleep 10 # wait for kubectl to become available

    echo "nodes:"
    kubectl get nodes -o wide
    echo "pods cidr: $pod_cidr"
    kubectl get pods -A -o wide
    echo "services cidr: $service_cidr"
    kubectl get services -A
}

export EMBEDDED_CLUSTER_METRICS_BASEURL="https://staging.replicated.app"
export KUBECONFIG=/var/lib/k0s/pki/admin.conf
export PATH=$PATH:/var/lib/embedded-cluster/bin
main "$@"
