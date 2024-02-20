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
    local installation_version=
    installation_version="$1"

    sleep 30 # wait for kubectl to become available

    local ec_version=
    ec_version=$(embedded-cluster version | grep AdminConsole | awk '{print substr($4,2)}')
    curl https://kots.io/install/$ec_version | bash

    echo "upgrading to version ${installation_version}-upgrade"
    kubectl kots upstream upgrade embedded-cluster-smoke-test-staging-app --namespace kotsadm --deploy-version-label="${installation_version}-upgrade"

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
    kubectl describe chart -n kube-system k0s-addon-chart-ingress-nginx | grep -q "test-upgrade-value" # ensure new values are present
    kubectl describe chart -n kube-system k0s-addon-chart-ingress-nginx | grep -q "4.9.1" # ensure new version is present
    kubectl describe pod -n ingress-nginx | grep -q "4.9.1" # ensure the new version made it into the pod

    # ensure that the embedded-cluster-operator has been updated
    kubectl describe chart -n kube-system k0s-addon-chart-embedded-cluster-operator
    kubectl describe chart -n kube-system k0s-addon-chart-embedded-cluster-operator | grep -q embedded-cluster-operator-upgrade-value # ensure new values are present
    kubectl describe pod -n embedded-cluster
    # ensure the new value made it into the pod
    if ! kubectl describe pod -n embedded-cluster | grep -q embedded-cluster-operator-upgrade-value ; then
        echo "embedded-cluster-operator-upgrade-value not found in embedded-cluster pod"
        kubectl logs -n embedded-cluster -l app.kubernetes.io/name=embedded-cluster-operator --tail=100
        exit 1
    fi

    echo "ensure that the admin console branding is available"
    kubectl get cm -n kotsadm kotsadm-application-metadata

    echo "ensure that the default chart order remained 10"
    kubectl describe clusterconfig -n kube-system k0s
    if ! kubectl describe clusterconfig -n kube-system k0s | grep "Order" | grep -q "10"; then
        echo "no charts had an order of '10'"
        exit 1
    fi
}

export EMBEDDED_CLUSTER_METRICS_BASEURL="https://staging.replicated.app"
export KUBECONFIG=/var/lib/k0s/pki/admin.conf
export PATH=$PATH:/var/lib/embedded-cluster/bin
main "$@"
