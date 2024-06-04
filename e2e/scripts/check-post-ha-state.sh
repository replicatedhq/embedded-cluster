#!/usr/bin/env bash
set -euox pipefail

wait_for_nginx_pods() {
    ready=$(kubectl get pods -n kotsadm | grep "nginx" | grep -c Running || true)
    counter=0
    while [ "$ready" -lt "1" ]; do
        if [ "$counter" -gt 36 ]; then
            echo "nginx pods did not appear"
            kubectl get pods -n kotsadm
            kubectl logs -n kotsadm -l app=kotsadm
            return 1
        fi
        sleep 5
        counter=$((counter+1))
        echo "Waiting for nginx pods"
        ready=$(kubectl get pods -n kotsadm | grep "nginx" | grep -c Running || true)
        kubectl get pods -n nginx 2>&1 || true
        echo "ready: $ready"
    done
}

ensure_app_deployed() {
    local version="$1"

    kubectl kots get versions -n kotsadm embedded-cluster-smoke-test-staging-app
    if ! kubectl kots get versions -n kotsadm embedded-cluster-smoke-test-staging-app | grep -q "${version}\W*[01]\W*deployed"; then
        echo "application version ${version} not deployed"
        return 1
    fi
}

ensure_app_not_upgraded() {
    if kubectl get ns | grep -q memcached ; then
        echo "found memcached ns"
        return 1
    fi
    if kubectl get pods -n kotsadm -l app=second | grep -q second ; then
        echo "found pods from app update"
        return 1
    fi
}

function retry() {
    local retries=$1
    shift

    local count=0
    until "$@"; do
        exit=$?
        wait=$((2 ** $count))
        count=$(($count + 1))
        if [ $count -lt $retries ]; then
            echo "Retry $count/$retries exited $exit, retrying in $wait seconds..."
            sleep $wait
        else
            echo "Retry $count/$retries exited $exit, no more retries left."
            return $exit
        fi
    done
    return 0
}

main() {
    local version="$1"
    sleep 10 # wait for kubectl to become available

    echo "pods"
    kubectl get pods -A

    kubectl get installations
    kubectl describe installations

    echo "ensure that installation is installed"
    kubectl get installations --no-headers | grep -q "Installed"

    # ensure rqlite is running in HA mode
    kubectl get sts -n kotsadm kotsadm-rqlite -o jsonpath='{.status.readyReplicas}' | grep -q 3

    if ! wait_for_nginx_pods; then
        echo "Failed waiting for the application's nginx pods"
        exit 1
    fi
    if ! retry 5 ensure_app_deployed "$version" ; then
        echo "Failed ensuring app is deployed"
        exit 1
    fi
    if ! ensure_app_not_upgraded; then
        echo "Failed ensuring app is not upgraded"
        exit 1
    fi

    echo "ensure that the admin console branding is available and has the DR label"
    if ! kubectl get cm -n kotsadm -l replicated.com/disaster-recovery=infra | grep -q kotsadm-application-metadata; then
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
