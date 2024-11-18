#!/usr/bin/env bash
set -euox pipefail

DIR=/usr/local/bin
. $DIR/common.sh

main() {
    local version="appver-$1"
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

    # if this is the current version in CI
    if echo "$version" | grep -qvE "(pre-minio-removal|1.8.0-k8s)" ; then
        validate_data_dirs
    fi

    # scale up the second deployment to ensure that images can still be pulled
    echo "scaling up the 'second' deployment to ensure that images can still be pulled"
    kubectl scale deployment/second --replicas=4
    sleep 5
    echo "after 5 seconds, pods in the 'kotsadm' namespace:"
    kubectl get pods -n kotsadm -o wide
    if ! wait_for_pods_running 60; then
        echo "Failed waiting for the second deployment's nginx pods"
        exit 1
    fi

    validate_no_pods_in_crashloop
}

main "$@"
