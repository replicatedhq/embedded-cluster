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
    echo "checking rqlite HA mode (expecting 3 ready replicas)"
    if ! kubectl get sts -n kotsadm kotsadm-rqlite -o jsonpath='{.status.readyReplicas}' | grep -q 3; then
        echo "rqlite is not running in HA mode (expected 3 ready replicas)"
        kubectl get sts -n kotsadm kotsadm-rqlite -o jsonpath='{.status.readyReplicas}'
        exit 1
    fi

    # ensure registry is running in HA mode
    echo "checking registry HA mode (expecting 2 ready replicas)"
    if ! kubectl get deployment -n registry registry -o jsonpath='{.status.readyReplicas}' | grep -q 2; then
        echo "registry is not running in HA mode (expected 2 ready replicas)"
        kubectl get deployment -n registry registry -o jsonpath='{.status.readyReplicas}'
        exit 1
    fi

    # ensure seaweedfs components are healthy
    echo "checking seaweedfs-filer HA mode (expecting 3 ready replicas)"
    if ! kubectl get statefulset -n seaweedfs seaweedfs-filer -o jsonpath='{.status.readyReplicas}' | grep -q 3; then
        echo "seaweedfs-filer is not running in HA mode (expected 3 ready replicas)"
        kubectl get statefulset -n seaweedfs seaweedfs-filer -o jsonpath='{.status.readyReplicas}'
        exit 1
    fi
    
    echo "checking seaweedfs-volume HA mode (expecting 3 ready replicas)"
    if ! kubectl get statefulset -n seaweedfs seaweedfs-volume -o jsonpath='{.status.readyReplicas}' | grep -q 3; then
        echo "seaweedfs-volume is not running in HA mode (expected 3 ready replicas)"
        kubectl get statefulset -n seaweedfs seaweedfs-volume -o jsonpath='{.status.readyReplicas}'
        exit 1
    fi
    
    echo "checking seaweedfs-master HA mode (expecting 3 ready replicas)"
    if ! kubectl get statefulset -n seaweedfs seaweedfs-master -o jsonpath='{.status.readyReplicas}' | grep -q 3; then
        echo "seaweedfs-master is not running in HA mode (expected 3 ready replicas)"
        kubectl get statefulset -n seaweedfs seaweedfs-master -o jsonpath='{.status.readyReplicas}'
        exit 1
    fi

    if [ "$from_restore" == "true" ]; then
        # ensure volumes were restored
        echo "checking kotsadm volume restore"
        if ! kubectl get podvolumerestore -n velero | grep kotsadm | grep -c backup | grep -q 1; then
            echo "kotsadm volumes were not properly restored"
            kubectl get podvolumerestore -n velero | grep kotsadm || true
            exit 1
        fi
        
        echo "checking seaweedfs-filer data volume restores"
        if ! kubectl get podvolumerestore -n velero | grep seaweedfs-filer | grep -c data-filer | grep -q 3; then
            echo "seaweedfs-filer data volumes were not properly restored (expected 3)"
            kubectl get podvolumerestore -n velero | grep seaweedfs-filer | grep data-filer || true
            exit 1
        fi
        
        echo "checking seaweedfs-filer log volume restores"
        if ! kubectl get podvolumerestore -n velero | grep seaweedfs-filer | grep -c seaweedfs-filer-log-volume | grep -q 3; then
            echo "seaweedfs-filer log volumes were not properly restored (expected 3)"
            kubectl get podvolumerestore -n velero | grep seaweedfs-filer | grep seaweedfs-filer-log-volume || true
            exit 1
        fi
        
        echo "checking seaweedfs-volume data restores"
        if ! kubectl get podvolumerestore -n velero | grep seaweedfs-volume | grep -c data | grep -q 3; then
            echo "seaweedfs-volume data volumes were not properly restored (expected 3)"
            kubectl get podvolumerestore -n velero | grep seaweedfs-volume | grep data || true
            exit 1
        fi
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

    # scale up the second deployment to ensure that images can still be pulled
    echo "scaling up the 'second' deployment to ensure that images can still be pulled"
    kubectl scale -n "$APP_NAMESPACE" deployment/second --replicas=4
    echo "waiting for the second deployment to scale up"
    for _ in {1..60}; do
        if kubectl get pods -n "$APP_NAMESPACE" | grep -q "second-"; then
            break
        fi
        sleep 1
    done
    echo "after 5 seconds, pods in the '$APP_NAMESPACE' namespace:"
    kubectl get pods -n "$APP_NAMESPACE" -o wide
    if ! wait_for_pods_running 60; then
        echo "Failed waiting for the second deployment's nginx pods"
        exit 1
    fi
    # scale the second deployment back down so that they aren't restored in the DR test
    kubectl scale -n "$APP_NAMESPACE" deployment/second --replicas=0

    validate_all_pods_healthy
}

main "$@"
