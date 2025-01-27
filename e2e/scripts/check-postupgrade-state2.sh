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

main() {
    local version="$1"
    local k8s_version="$2"

    echo "TODO: check installation configmap state"

    echo "pods"
    kubectl get pods -A

    # the airgap version of this command does not require the kots CLI, which is why it is used here
    if ! ensure_app_deployed_airgap "$version"; then
        echo "failed to find that version $version was deployed"
        exit 1
    fi

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
    if ! retry 5 eval "kubectl get pods -n $APP_NAMESPACE -l app=second | grep -q Running" ; then
        echo "no pods found for second app version"
        kubectl get pods -n "$APP_NAMESPACE"
        exit 1
    fi

    # ensure that nginx-ingress has been updated
    if ! retry 5 check_nginx_version ; then
        echo "4.12.0-beta.0 not found in ingress-nginx pod"
        kubectl describe pod -n ingress-nginx
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

    validate_no_pods_in_crashloop
}

main "$@"
