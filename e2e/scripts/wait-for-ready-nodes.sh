#!/usr/bin/env bash
# This script waits for X nodes to be ready. X is the first argument.
set -euox pipefail

wait_for_dns_ready() {
    local url="$1"
    local max_retries=10
    local retries=0
    while [ "$retries" -lt "$max_retries" ]; do
        if curl -s -o /dev/null -I -w "%{http_code}" "$url" | grep -q 200; then
            return 0
        fi
        retries=$((retries+1))
        sleep 10
    done
    return 1
}

main() {
    expected_nodes="$1"
    is_restore="${2:-}"

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

    if [ "$is_restore" == "true" ]; then
        # this is a restore operation where the app hasn't been restored yet, so goldpinger won't exist
        # wait for DNS lookup to work. (this shouldn't be necessary, but CI breaks without it)
        wait_for_dns_ready "https://staging.replicated.app"
        exit 0
    fi

    echo "checking that goldpinger has run on all nodes"
    kubectl get pods -n goldpinger
    local goldpinger_running_count=
    goldpinger_running_count=$(kubectl get pods --no-headers -n goldpinger | wc -l)
    if [ "$goldpinger_running_count" -lt "$expected_nodes" ]; then
        echo "goldpinger is running on $goldpinger_running_count nodes, expected $expected_nodes"
        exit 1
    fi

    exit 0
}

export EMBEDDED_CLUSTER_METRICS_BASEURL="https://staging.replicated.app"
export KUBECONFIG=/var/lib/k0s/pki/admin.conf
export PATH=$PATH:/var/lib/embedded-cluster/bin
main "$@"
