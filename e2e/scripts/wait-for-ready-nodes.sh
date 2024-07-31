#!/usr/bin/env bash
# This script waits for X nodes to be ready. X is the first argument.
set -euox pipefail

main() {
    expected_nodes="$1"
    is_restore="${2:-}"

    ready=$(kubectl get nodes | grep -v NotReady | grep -c Ready || true)
    counter=0
    while [ "$ready" -lt "$expected_nodes" ]; do
        echo "Waiting for nodes to be ready ($ready/$expected_nodes)"
        if [ "$counter" -gt 48 ]; then
            echo "Timed out waiting for $expected_nodes nodes to be ready"
            exit 1
        fi
        sleep 5
        counter=$((counter+1))
        ready=$(kubectl get nodes | grep -v NotReady | grep -c Ready || true)
        kubectl get nodes -o wide || true
    done
    echo "All nodes are ready"

    if [ "$is_restore" == "true" ]; then
        # this is a restore operation where the app hasn't been restored yet, so goldpinger won't exist
        exit 0
    fi

    echo "checking that goldpinger is running on all nodes"
    kubectl get pods -n goldpinger
    goldpinger_ready=$(kubectl get pods --no-headers -n goldpinger | grep 'Running' | grep '1/1' | wc -l)
    counter=0
    while [ "$goldpinger_ready" -lt "$expected_nodes" ]; do
        echo "goldpinger is running on $goldpinger_ready nodes, expected $expected_nodes"
        if [ "$counter" -gt 48 ]; then
            echo "Timed out waiting for goldpinger to be running on $expected_nodes nodes"
            kubectl get nodes -o wide
            kubectl get pods -n goldpinger -o wide
            kubectl describe pods -n goldpinger
            exit 1
        fi
        sleep 5
        counter=$((counter+1))
        goldpinger_ready=$(kubectl get pods --no-headers -n goldpinger | grep 'Running' | grep '1/1' | wc -l)
        kubectl get pods -n goldpinger || true
    done
    echo "goldpinger is running on all nodes"

    exit 0
}

export EMBEDDED_CLUSTER_METRICS_BASEURL="https://staging.replicated.app"
export KUBECONFIG=/var/lib/k0s/pki/admin.conf
export PATH=$PATH:/var/lib/embedded-cluster/bin
builtin alias kubectl='k0s kubectl'

main "$@"
