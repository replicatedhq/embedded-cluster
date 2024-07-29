#!/usr/bin/env bash
set -euox pipefail

DIR=/usr/local/bin
. $DIR/common.sh

main() {
    local version="appver-$1"
    local from_restore="${2:-}"
    sleep 10 # wait for kubectl to become available

    echo "pods"
    kubectl get pods -A

    kubectl get installations
    kubectl describe installations

    echo "ensure that installation is installed"
    kubectl get installations --no-headers | grep -q "Installed"

    # ensure rqlite is running in HA mode
    kubectl get sts -n kotsadm kotsadm-rqlite -o jsonpath='{.status.readyReplicas}' | grep -q 3

    # ensure registry is running in HA mode
    kubectl get deployment -n registry registry -o jsonpath='{.status.readyReplicas}' | grep -q 2

    # ensure seaweedfs components are healthy
    kubectl get statefulset -n seaweedfs seaweedfs-filer -o jsonpath='{.status.readyReplicas}' | grep -q 3
    kubectl get statefulset -n seaweedfs seaweedfs-volume -o jsonpath='{.status.readyReplicas}' | grep -q 3
    kubectl get statefulset -n seaweedfs seaweedfs-master -o jsonpath='{.status.readyReplicas}' | grep -q 1

    if [ "$from_restore" == "true" ]; then
        # ensure volumes were restored
        kubectl get podvolumerestore -n velero | grep kotsadm | grep -c backup | grep -q 1
        kubectl get podvolumerestore -n velero | grep seaweedfs-filer | grep -c data-filer | grep -q 3
        kubectl get podvolumerestore -n velero | grep seaweedfs-filer | grep -c seaweedfs-filer-log-volume | grep -q 3
        kubectl get podvolumerestore -n velero | grep seaweedfs-volume | grep -c data | grep -q 3
    fi

    if ! wait_for_nginx_pods; then
        echo "Failed waiting for the application's nginx pods"
        exit 1
    fi
    if ! ensure_app_deployed_airgap "$version"; then
        echo "Failed ensuring app is deployed"
        exit 1
    fi
    if ! ensure_app_not_upgraded; then
        echo "Failed ensuring app is not upgraded"
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
export PATH=$PATH:/var/lib/embedded-cluster/bin
main "$@"
