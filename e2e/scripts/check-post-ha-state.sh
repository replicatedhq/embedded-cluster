#!/usr/bin/env bash
set -euox pipefail

DIR=/usr/local/bin
. $DIR/common.sh

main() {
    local version="$1"
    local k8s_version="$2"
    local from_restore="${2:-}"

    sleep 10 # wait for kubectl to become available

    echo "ensure that all nodes are running k8s $k8s_version"
    if ! ensure_nodes_match_kube_version "$k8s_version"; then
        echo "not all nodes are running k8s $k8s_version"
        exit 1
    fi

    echo "pods"
    kubectl get pods -A

    kubectl get installations
    kubectl describe installations

    if ! ensure_installation_is_installed; then
        echo "installation is not installed"
        exit 1
    fi

    # ensure rqlite is running in HA mode
    if ! kubectl get sts -n kotsadm kotsadm-rqlite -o jsonpath='{.status.readyReplicas}' | grep -q 3; then
        echo "kotsadm-rqlite is not ready and HA"
        kubectl get sts -n kotsadm kotsadm-rqlite
        exit 1
    fi

    if [ "$from_restore" == "true" ]; then
        # ensure volumes were restored
        kubectl get podvolumerestore -n velero | grep kotsadm | grep -c backup | grep -q 1
    fi

    if ! wait_for_nginx_pods; then
        echo "Failed waiting for the application's nginx pods"
        exit 1
    fi
    if ! ensure_app_deployed "$version"; then
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
