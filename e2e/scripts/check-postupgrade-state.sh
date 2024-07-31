#!/usr/bin/env bash
set -euox pipefail

DIR=/usr/local/bin
. $DIR/common.sh

function check_nginx_version {
    if ! kubectl describe pod -n ingress-nginx | grep -q "4.9.1"; then
        return 1
    fi
    return 0
}

main() {
    local k8s_version="$1"

    echo "ensure that installation is installed"
    wait_for_installation

    kubectl get installations --no-headers | grep -q "Installed"

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
    if ! kubectl get pods -n kotsadm -l app=second | grep -q Running ; then
        echo "no pods found for second app version"
        kubectl get pods -n kotsadm
        exit 1
    fi

    # ensure that nginx-ingress has been updated
    kubectl describe chart -n kube-system k0s-addon-chart-ingress-nginx
    # ensure new values are present
    if ! kubectl describe chart -n kube-system k0s-addon-chart-ingress-nginx | grep -q "test-upgrade-value"; then
        echo "test-upgrade-value not found in ingress-nginx chart"
        exit 1
    fi
    # ensure new version is present
    if ! kubectl describe chart -n kube-system k0s-addon-chart-ingress-nginx | grep -q "4.9.1"; then
        echo "4.9.1 not found in ingress-nginx chart"
        exit 1
    fi
    # ensure the new version made it into the pod
    if ! retry 5 check_nginx_version ; then
        echo "4.9.1 not found in ingress-nginx pod"
        kubectl describe pod -n ingress-nginx
        exit 1
    fi

    # ensure that the embedded-cluster-operator has been updated
    kubectl describe chart -n kube-system k0s-addon-chart-embedded-cluster-operator
    kubectl describe chart -n kube-system k0s-addon-chart-embedded-cluster-operator | grep -q "123m" # ensure new values are present
    kubectl describe pod -n embedded-cluster
    # ensure the new value made it into the pod
    if ! kubectl describe pod -n embedded-cluster | grep -q "123m" ; then
        echo "CPU request of 123m not found in embedded-cluster pod"
        kubectl logs -n embedded-cluster -l app.kubernetes.io/name=embedded-cluster-operator --tail=100
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

    echo "ensure that the default chart order remained 110"
    if ! kubectl describe clusterconfig -n kube-system k0s | grep -q -e 'Order:\W*110' ; then
        kubectl describe clusterconfig -n kube-system k0s
        echo "no charts had an order of '110'"
        exit 1
    fi

    echo "ensure that all nodes are running k8s $k8s_version"
    if ! ensure_nodes_match_kube_version "$k8s_version"; then
        echo "not all nodes are running k8s $k8s_version"
        exit 1
    fi
}

export EMBEDDED_CLUSTER_METRICS_BASEURL="https://staging.replicated.app"
export KUBECONFIG=/var/lib/k0s/pki/admin.conf
export PATH=$PATH:/var/lib/embedded-cluster/bin
builtin alias kubectl='k0s kubectl'

main "$@"
