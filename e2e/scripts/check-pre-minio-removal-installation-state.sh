#!/usr/bin/env bash
set -euox pipefail

wait_for_installation() {
    ready=$(kubectl get installations --no-headers | grep -c "Installed" || true)
    counter=0
    while [ "$ready" -lt "1" ]; do
        if [ "$counter" -gt 36 ]; then
            echo "installation did not become ready"
            kubectl get installations 2>&1 || true
            kubectl describe installations 2>&1 || true
            kubectl get charts -A
            kubectl describe chart -n kube-system k0s-addon-chart-ingress-nginx
            kubectl get secrets -A
            kubectl describe clusterconfig -A
            echo "operator logs:"
            kubectl logs -n embedded-cluster -l app.kubernetes.io/name=embedded-cluster-operator --tail=100
            return 1
        fi
        sleep 5
        counter=$((counter+1))
        echo "Waiting for installation"
        ready=$(kubectl get installations --no-headers | grep -c "Installed" || true)
        kubectl get installations 2>&1 || true
    done
}

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

main() {
    local version="$1"
    sleep 30 # wait for kubectl to become available

    echo "pods"
    kubectl get pods -A

    echo "ensure that installation is installed"
    wait_for_installation
    kubectl get installations --no-headers | grep -q "Installed"

    echo "ensure that the admin console branding is available"
    kubectl get cm -n kotsadm kotsadm-application-metadata

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
}

export EMBEDDED_CLUSTER_METRICS_BASEURL="https://staging.replicated.app"
export KUBECONFIG=/var/lib/k0s/pki/admin.conf
export PATH=$PATH:/var/lib/embedded-cluster/bin
main "$@"
