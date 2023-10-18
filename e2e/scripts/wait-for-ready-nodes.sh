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

export HELMVM_METRICS_BASEURL="https://staging.replicated.app"
export KUBECONFIG=/root/.config/helmvm/etc/kubeconfig
export PATH=$PATH:/root/.config/helmvm/bin
main "$@"
