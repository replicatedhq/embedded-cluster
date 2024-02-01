#!/usr/bin/env bash
set -euo pipefail

main() {
    sleep 30

    echo "pods"
    kubectl get pods -A
    echo "installations"
    kubectl get installations
    kubectl describe installations
    echo "charts"
    kubectl get charts -A
    kubectl describe charts -A

    echo "ensure that installation is installed"
    kubectl get installations -o jsonpath='{.items[0].status.state}' | grep -q "Installed"
}

export EMBEDDED_CLUSTER_METRICS_BASEURL="https://staging.replicated.app"
export KUBECONFIG=/root/.config/embedded-cluster/etc/kubeconfig
export PATH=$PATH:/root/.config/embedded-cluster/bin
main
