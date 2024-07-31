#!/usr/bin/env bash
set -euox pipefail

wait_for_velero_pods() {
    ready=$(kubectl get pods -n velero -o jsonpath='{.items[*].metadata.name} {.items[*].status.phase}' | grep "velero" | grep -c Running || true)
    counter=0
    while [ "$ready" -lt "1" ]; do
        if [ "$counter" -gt 36 ]; then
            echo "velero pods did not appear"
            kubectl get pods -n velero -o jsonpath='{.items[*].metadata.name} {.items[*].status.phase}'
            kubectl get pods -n velero
            return 1
        fi
        sleep 5
        counter=$((counter+1))
        echo "Waiting for velero pods"
        ready=$(kubectl get pods -n velero -o jsonpath='{.items[*].metadata.name} {.items[*].status.phase}' | grep "velero" | grep -c Running || true)
        kubectl get pods -n velero 2>&1 || true
        echo "ready: $ready"
    done
}

main() {
    sleep 50

    kubectl get pods -A
    kubectl get installations -o yaml
    kubectl get charts -A

    if ! wait_for_velero_pods; then
        echo "Failed waiting for velero"
        exit 1
    fi
}

export EMBEDDED_CLUSTER_METRICS_BASEURL="https://staging.replicated.app"
export KUBECONFIG=/var/lib/k0s/pki/admin.conf
export PATH=\$PATH:/var/lib/embedded-cluster/bin
main "$@"
