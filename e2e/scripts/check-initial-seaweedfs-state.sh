#!/usr/bin/env bash
set -euox pipefail

DIR=/usr/local/bin
. $DIR/common.sh

main() {
    sleep 10 # wait for kubectl to become available

    echo "pods"
    kubectl get pods -A

    kubectl get installations
    kubectl describe installations

    if ! ensure_installation_is_installed; then
        echo "installation is not installed"
        exit 1
    fi

    # ensure seaweedfs-master is running in single-replica mode (pre-2.8.1 behavior)
    echo "checking seaweedfs-master single replica mode (expecting 1 ready replica)"
    if ! kubectl get statefulset -n seaweedfs seaweedfs-master -o jsonpath='{.status.readyReplicas}' | grep -q 1; then
        echo "seaweedfs-master is not running in single replica mode (expected 1 ready replica)"
        kubectl get statefulset -n seaweedfs seaweedfs-master -o jsonpath='{.status.readyReplicas}'
        exit 1
    fi

    # verify that the spec also shows 1 replica
    echo "checking seaweedfs-master spec replicas (expecting 1)"
    if ! kubectl get statefulset -n seaweedfs seaweedfs-master -o jsonpath='{.spec.replicas}' | grep -q 1; then
        echo "seaweedfs-master spec does not show 1 replica"
        kubectl get statefulset -n seaweedfs seaweedfs-master -o jsonpath='{.spec.replicas}'
        exit 1
    fi

    # check that seaweedfs-filer and seaweedfs-volume are also running in single-replica mode
    echo "checking seaweedfs-filer single replica mode (expecting 1 ready replica)"
    if ! kubectl get statefulset -n seaweedfs seaweedfs-filer -o jsonpath='{.status.readyReplicas}' | grep -q 1; then
        echo "seaweedfs-filer is not running in single replica mode (expected 1 ready replica)"
        kubectl get statefulset -n seaweedfs seaweedfs-filer -o jsonpath='{.status.readyReplicas}'
        exit 1
    fi

    echo "checking seaweedfs-volume single replica mode (expecting 1 ready replica)"
    if ! kubectl get statefulset -n seaweedfs seaweedfs-volume -o jsonpath='{.status.readyReplicas}' | grep -q 1; then
        echo "seaweedfs-volume is not running in single replica mode (expected 1 ready replica)"
        kubectl get statefulset -n seaweedfs seaweedfs-volume -o jsonpath='{.status.readyReplicas}'
        exit 1
    fi

    echo "seaweedfs is correctly running in single-replica mode (pre-2.8.1 behavior)"
}

main "$@"