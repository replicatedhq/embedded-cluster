#!/usr/bin/env bash
set -euo pipefail

wait_for_installation() {
    ready=$(kubectl get installations --no-headers | grep -c "Installed" || true)
    counter=0
    while [ "$ready" -lt "1" ]; do
        if [ "$counter" -gt 36 ]; then
            echo "installation did not become ready"
            kubectl get installations 2>&1 || true
            kubectl describe installations 2>&1 || true
            kubectl get charts -A
            kubectl get secrets -A
            kubectl describe clusterconfig -A
            kubectl get pods -A
            echo "operator logs:"
            kubectl logs -n embedded-cluster -l app.kubernetes.io/name=embedded-cluster-operator
            return 1
        fi
        sleep 5
        counter=$((counter+1))
        echo "Waiting for installation"
        ready=$(kubectl get installations --no-headers | grep -c "Installed" || true)
        kubectl get installations 2>&1 || true
    done
}

main() {
    sleep 30 # wait for kubectl to become available

    curl https://kots.io/install/1.107.1 | bash

    kubectl kots upstream upgrade embedded-cluster-smoke-test-staging-app --deploy-version-label="0.1.10" --namespace kotsadm

    sleep 30

    echo "ensure that installation is installed"
    wait_for_installation
    kubectl get installations --no-headers | grep -q "Installed"

    echo "pods"
    kubectl get pods -A
    echo "charts"
    kubectl get charts -A
    echo "installations"
    kubectl get installations

    # ensure that goldpinger exists
    kubectl get ns goldpinger

    # ensure that new app pods exist
    kubectl get pods -n kotsadm -l app=second

    # ensure that nginx-ingress has been updated
    kubectl describe chart -n kube-system k0s-addon-chart-ingress-nginx
    kubectl describe pods -n ingress-nginx
    kubectl get clusterconfig -n kube-system k0s -o yaml
    kubectl get clusterconfig -n kube-system k0s -o yaml | grep -q "test-upgrade-value"
    kubectl describe pods -n ingress-nginx | grep -q "test-upgrade-value"
}

export EMBEDDED_CLUSTER_METRICS_BASEURL="https://staging.replicated.app"
export KUBECONFIG=/root/.config/embedded-cluster/etc/kubeconfig
export PATH=$PATH:/root/.config/embedded-cluster/bin
main
