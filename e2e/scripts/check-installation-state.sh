#!/usr/bin/env bash
set -euo pipefail

main() {
    sleep 60
    echo "installations"
    kubectl get installations
    kubectl describe installations
    kubectl get installations -o yaml
    echo "charts"
    kubectl get charts -A
    kubectl describe charts -A
    kubectl get charts -A -o yaml
}

export EMBEDDED_CLUSTER_METRICS_BASEURL="https://staging.replicated.app"
export KUBECONFIG=/root/.config/embedded-cluster/etc/kubeconfig
export PATH=$PATH:/root/.config/embedded-cluster/bin
main
