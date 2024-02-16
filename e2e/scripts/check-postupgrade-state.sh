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
            /usr/local/bin/k0s kubectl get secrets -A
            /usr/local/bin/k0s kubectl describe clusterconfig -A
            /usr/local/bin/k0s kubectl get pods -A
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
    local installation_version=
    installation_version="$1"

    sleep 30 # wait for /usr/local/bin/k0s kubectl to become available

    local ec_version=
    ec_version=$(embedded-cluster version | grep AdminConsole | awk '{print substr($4,2)}')
    curl https://kots.io/install/$ec_version | bash

    echo "upgrading to version ${installation_version}-upgrade"
    /usr/local/bin/k0s kubectl kots upstream upgrade embedded-cluster-smoke-test-staging-app --namespace kotsadm --deploy-version-label="${installation_version}-upgrade"

    sleep 30

    echo "ensure that installation is installed"
    wait_for_installation

    /usr/local/bin/k0s kubectl get installations --no-headers | grep -q "Installed"

    echo "pods"
    /usr/local/bin/k0s kubectl get pods -A
    echo "charts"
    /usr/local/bin/k0s kubectl get charts -A
    echo "installations"
    /usr/local/bin/k0s kubectl get installations

    # ensure that goldpinger exists
    /usr/local/bin/k0s kubectl get ns goldpinger

    # ensure that new app pods exist
    /usr/local/bin/k0s kubectl get pods -n kotsadm -l app=second

    # ensure that nginx-ingress has been updated
    /usr/local/bin/k0s kubectl describe chart -n kube-system k0s-addon-chart-ingress-nginx
    /usr/local/bin/k0s kubectl describe chart -n kube-system k0s-addon-chart-ingress-nginx | grep -q "test-upgrade-value" # ensure new values are present
    /usr/local/bin/k0s kubectl describe chart -n kube-system k0s-addon-chart-ingress-nginx | grep -q "4.9.1" # ensure new version is present
    /usr/local/bin/k0s kubectl describe pod -n ingress-nginx | grep -q "4.9.1" # ensure the new version made it into the pod

    # ensure that the embedded-cluster-operator has been updated
    /usr/local/bin/k0s kubectl describe chart -n kube-system k0s-addon-chart-embedded-cluster-operator
    /usr/local/bin/k0s kubectl describe chart -n kube-system k0s-addon-chart-embedded-cluster-operator | grep -q embedded-cluster-operator-upgrade-value # ensure new values are present
    /usr/local/bin/k0s kubectl describe pod -n embedded-cluster
    # ensure the new value made it into the pod
    if ! /usr/local/bin/k0s kubectl describe pod -n embedded-cluster | grep -q embedded-cluster-operator-upgrade-value ; then
        echo "embedded-cluster-operator-upgrade-value not found in embedded-cluster pod"
        /usr/local/bin/k0s kubectl logs -n embedded-cluster -l app.kubernetes.io/name=embedded-cluster-operator --tail=100
        exit 1
    fi
}

export EMBEDDED_CLUSTER_METRICS_BASEURL="https://staging.replicated.app"
export KUBECONFIG=/root/.config/embedded-cluster/etc/kubeconfig
alias kubectl="/usr/local/bin/k0s kubectl"
export PATH=$PATH:/root/.config/embedded-cluster/bin
main "$@"
