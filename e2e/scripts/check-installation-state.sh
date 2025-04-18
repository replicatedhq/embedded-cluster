#!/usr/bin/env bash
set -euox pipefail

DIR=/usr/local/bin
. $DIR/common.sh

main() {
    local version="$1"
    local k8s_version="$2"

    sleep 10 # wait for kubectl to become available

    echo "ensure that all nodes are running k8s $k8s_version"
    if ! ensure_nodes_match_kube_version "$k8s_version"; then
        echo "not all nodes are running k8s $k8s_version"
        exit 1
    fi

    echo "pods"
    kubectl get pods -A

    echo "ensure that installation is installed"
    if echo "$version" | grep "pre-minio-removal"; then
        echo "waiting for installation as this is a pre-minio-removal embedded-cluster version (and so the installer doesn't wait for the installation to be ready itself)"
        wait_for_installation
    fi
    if ! ensure_installation_is_installed; then
        echo "installation is not installed"
        exit 1
    fi

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

    # if this is the current version in CI
    if echo "$version" | grep -qvE "(pre-minio-removal|1.8.0-k8s|previous-stable)" ; then
        validate_data_dirs
    fi

    # do not run this test with previous versions of the embedded-cluster binary - the command is new
    # this check is different than the one above as we don't want to run it even after we have upgraded the cluster,
    # as that doesn't upgrade the embedded-cluster binary on this node
    if embedded-cluster version | grep -qvE "(pre-minio-removal|1.8.0-k8s|previous-stable)"  ; then
        check_join_command
    fi

    validate_no_pods_in_crashloop
}

main "$@"
