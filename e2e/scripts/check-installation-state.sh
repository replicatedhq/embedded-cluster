#!/usr/bin/env bash
set -euo pipefail

wait_for_installation() {
    ready=$(/usr/local/bin/k0s kubectl get installations --no-headers | grep -c "Installed" || true)
    counter=0
    while [ "$ready" -lt "1" ]; do
        if [ "$counter" -gt 36 ]; then
            echo "installation did not become ready"
            /usr/local/bin/k0s kubectl get installations 2>&1 || true
            /usr/local/bin/k0s kubectl describe installations 2>&1 || true
            /usr/local/bin/k0s kubectl get charts -A
            /usr/local/bin/k0s kubectl describe chart -n kube-system k0s-addon-chart-ingress-nginx
            /usr/local/bin/k0s kubectl get secrets -A
            /usr/local/bin/k0s kubectl describe clusterconfig -A
            echo "operator logs:"
            /usr/local/bin/k0s kubectl logs -n embedded-cluster -l app.kubernetes.io/name=embedded-cluster-operator --tail=100
            return 1
        fi
        sleep 5
        counter=$((counter+1))
        echo "Waiting for installation"
        ready=$(/usr/local/bin/k0s kubectl get installations --no-headers | grep -c "Installed" || true)
        /usr/local/bin/k0s kubectl get installations 2>&1 || true
    done
}

main() {
    sleep 30 # wait for /usr/local/bin/k0s kubectl to become available

    echo "pods"
    /usr/local/bin/k0s kubectl get pods -A

    echo "ensure that installation is installed"
    wait_for_installation
    /usr/local/bin/k0s kubectl get installations --no-headers | grep -q "Installed"
}

export EMBEDDED_CLUSTER_METRICS_BASEURL="https://staging.replicated.app"
export KUBECONFIG=/root/.config/embedded-cluster/etc/kubeconfig
export PATH=$PATH:/root/.config/embedded-cluster/bin
main
