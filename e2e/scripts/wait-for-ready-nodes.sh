#!/usr/bin/env bash
# This script waits for X nodes to be ready. X is the first argument.
set -euo pipefail

main() {
    expected_nodes="$1"
    ready=$(kubectl get nodes | grep -v NotReady | grep -c Ready || true)
    counter=0
    while [ "$ready" -lt "$expected_nodes" ]; do
        echo "Waiting for nodes to be ready ($ready/$expected_nodes)"
        if [ "$counter" -gt 36 ]; then
            echo "Timed out waiting for $expected_nodes nodes to be ready"
            exit 1
        fi
        sleep 5
        counter=$((counter+1))
        ready=$(kubectl get nodes | grep -v NotReady | grep -c Ready || true)
        kubectl get nodes || true
    done
    echo "All nodes are ready"
    exit 0
}

export EMBEDDED_CLUSTER_METRICS_BASEURL="https://staging.replicated.app"
export KUBECONFIG=/root/.config/embedded-cluster/etc/kubeconfig
ln -s /usr/local/bin/k0s /usr/local/bin/kubectl
export PATH=$PATH:/root/.config/embedded-cluster/bin
main "$@"
