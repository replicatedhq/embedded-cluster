#!/usr/bin/env bash
set -euox pipefail

DIR=/usr/local/bin
. $DIR/common.sh

main() {
    local version="$1"
    sleep 10 # wait for kubectl to become available

    echo "pods"
    kubectl get pods -A

    echo "ensure that installation is installed"
    if echo "$version" | grep "pre-minio-removal"; then
        echo "waiting for installation as this is a pre-minio-removal embedded-cluster version (and so the installer doesn't wait for the installation to be ready itself)"
        wait_for_installation
    fi
    kubectl get installations --no-headers | grep -q "Installed"

    if ! wait_for_nginx_pods; then
        echo "Failed waiting for the application's nginx pods"
        exit 1
    fi
    if ! ensure_app_deployed "$version"; then
        exit 1
    fi
    if ! ensure_app_not_upgraded; then
        exit 1
    fi

    echo "ensure that the admin console branding is available and has the DR label"
    if ! kubectl get cm -n kotsadm kotsadm-application-metadata --show-labels | grep -q 'replicated.com/disaster-recovery=infra'; then
        echo "kotsadm-application-metadata configmap not found with the DR label"
        kubectl get cm -n kotsadm --show-labels
        kubectl get cm -n kotsadm kotsadm-application-metadata -o yaml
        exit 1
    fi
}

export EMBEDDED_CLUSTER_METRICS_BASEURL="https://staging.replicated.app"
export KUBECONFIG=/var/lib/k0s/pki/admin.conf
export PATH=\$PATH:/var/lib/embedded-cluster/bin
main "$@"
