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
            kubectl describe chart -n kube-system k0s-addon-chart-ingress-nginx
            kubectl get secrets -A
            kubectl describe clusterconfig -A
            echo "operator logs:"
            kubectl logs -n embedded-cluster -l app.kubernetes.io/name=embedded-cluster-operator --tail=100
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

    echo "pods"
    kubectl get pods -A

    echo "helm configs"
    sudo find /var -type f -print | grep '_helm_extension_'

    echo "ensure that installation is installed"
    wait_for_installation
    kubectl get installations --no-headers | grep -q "Installed"

    echo "ensure that the admin console branding is available"
    kubectl get cm -n kotsadm kotsadm-application-metadata
}

export EMBEDDED_CLUSTER_METRICS_BASEURL="https://staging.replicated.app"
export KUBECONFIG=/var/lib/k0s/pki/admin.conf
export PATH=$PATH:/var/lib/embedded-cluster/bin
main
