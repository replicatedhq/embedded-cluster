#!/usr/bin/env bash
set -euox pipefail

DIR=/usr/local/bin
. $DIR/common.sh

function check_nginx_version {
    if ! kubectl describe pod -n ingress-nginx | grep -q "4.12.0-beta.0"; then
        return 1
    fi
    return 0
}

function check_nginx_annotation {
    if ! kubectl describe service -n ingress-nginx ingress-nginx-controller | grep -q "test-upgrade-value"; then
        return 1
    fi
    return 0
}

main() {
    local k8s_version="$1"
    local ec_version="$2"

    echo "ensure that installation is installed"
    wait_for_installation

    echo "pods"
    kubectl get pods -A
    echo "charts"
    kubectl get charts -A
    echo "installations"
    kubectl get installations

    # ensure that memcached exists
    if ! kubectl get ns memcached; then
        echo "no memcached ns found"
        kubectl get ns
        exit 1
    fi

    # ensure that memcached pods exist
    if ! kubectl get pods -n memcached | grep -q Running ; then
        echo "no pods found for memcached deployment"
        kubectl get pods -n memcached
        exit 1
    fi

    # ensure that new app pods exist
    # wait for new app pods to be running
    # NOTE: we cannot use grep -q here as a SIGPIPE will cause the script to exit with 141 if
    # multiple lines are found, as grep -q will exit immediately and kubectl will still be writing
    # to the pipe
    if ! retry 5 eval "kubectl get pods -n $APP_NAMESPACE -l app=second | grep Running >/dev/null" ; then
        echo "no pods found for second app version"
        kubectl get pods -n "$APP_NAMESPACE"
        exit 1
    fi

    # ensure the new version made it into the pod
    if ! retry 5 check_nginx_version ; then
        echo "4.12.0-beta.0 not found in ingress-nginx pod"
        kubectl describe pod -n ingress-nginx
        exit 1
    fi
    # ensure that the new annotation made it into the service
    if ! retry 5 check_nginx_annotation ; then
        echo "test-upgrade-value not found in ingress-nginx-controller service"
        kubectl describe service -n ingress-nginx-controller
        exit 1
    fi

    # ensure that the overrides were applied as part of the upgrade
    if ! ensure_release_builtin_overrides_postupgrade ; then
        echo "Failed to validate that overrides were applied as part of the upgrade"
        exit 1
    fi

    # ensure that the embedded-cluster-operator has been updated
    kubectl describe pod -n embedded-cluster -l app.kubernetes.io/name=embedded-cluster-operator
    # ensure the new value made it into the pod
    if ! kubectl describe pod -n embedded-cluster -l app.kubernetes.io/name=embedded-cluster-operator | grep "EMBEDDEDCLUSTER_VERSION" | grep -q -e "$ec_version" ; then
        echo "Upgrade version not present in embedded-cluster-operator environment variable"
        kubectl logs -n kotsadm -l app.kubernetes.io/name=embedded-cluster-upgrade
        exit 1
    fi

    # TODO: validate that labels are added after upgrading from an older version
    echo "ensure that the admin console branding is available"
    kubectl get cm -n kotsadm kotsadm-application-metadata

    echo "ensure that the kotsadm deployment exists"
    kubectl get deployment -n kotsadm kotsadm

    echo "ensure the kotsadm statefulset does not exist"
    if kubectl get statefulset -n kotsadm kotsadm; then
        echo "kotsadm statefulset found"
        kubectl get statefulset -n kotsadm kotsadm
        exit 1
    fi

    echo "ensure the kotsadm-minio statefulset does not exist"
    if kubectl get statefulset -n kotsadm kotsadm-minio; then
        echo "kotsadm-minio statefulset found"
        kubectl get statefulset -n kotsadm kotsadm-minio
        exit 1
    fi

    echo "ensure that all nodes are running k8s $k8s_version"
    if ! ensure_nodes_match_kube_version "$k8s_version"; then
        echo "not all nodes are running k8s $k8s_version"
        exit 1
    fi

    validate_data_dirs

    validate_all_pods_healthy
}

main "$@"
