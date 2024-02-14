#!/usr/bin/env bash
set -euo pipefail

wait_for_healthy_node() {
    ready=$(kubectl get nodes | grep -v NotReady | grep -c Ready || true)
    counter=0
    while [ "$ready" -lt "1" ]; do
        if [ "$counter" -gt 36 ]; then
            return 1
        fi
        sleep 5
        counter=$((counter+1))
        echo "Waiting for node to be ready"
        ready=$(kubectl get nodes | grep -v NotReady | grep -c Ready || true)
        kubectl get nodes || true
    done

    return 0
}

wait_for_pods_running() {
    local timeout="$1"
    local start_time
    local current_time
    local elapsed_time
    start_time=$(date +%s)
    while true; do
        current_time=$(date +%s)
        elapsed_time=$((current_time - start_time))
        if [ "$elapsed_time" -ge "$timeout" ]; then
            kubectl get pods -A -o yaml || true
            kubectl describe nodes || true
            echo "Timed out waiting for all pods to be running."
            return 1
        fi
        local non_running_pods
        non_running_pods=$(kubectl get pods --all-namespaces --no-headers 2>/dev/null | awk '$4 != "Running" && $4 != "Completed" { print $0 }' | wc -l || echo 1)
        if [ "$non_running_pods" -ne 0 ]; then
            echo "Not all pods are running. Waiting."
            kubectl get pods,nodes -A || true
            sleep 5
            continue
        fi
        echo "All pods are running."
        return 0
    done
}

ensure_node_config() {
    if ! kubectl describe node | grep "controller-label" ; then
        echo "Failed to find controller-label"
        return 1
    fi

    if ! kubectl describe node | grep "controller-test" ; then
        echo "Failed to find controller-test"
        return 1
    fi
}

wait_for_ingress_pods() {
    ready=$(kubectl get pods -n ingress-nginx -o jsonpath='{.items[*].status.phase}' | grep -c Running || true)
    counter=0
    while [ "$ready" -lt "1" ]; do
        if [ "$counter" -gt 36 ]; then
            echo "ingress pods did not appear"
            kubectl get pods -n ingress-nginx -o jsonpath='{.items[*].status.phase}'
            kubectl get pods -n ingress-nginx 2>&1 || true
            kubectl get secrets -n ingress-nginx 2>&1 || true
            kubectl get charts -A
            return 1
        fi
        sleep 5
        counter=$((counter+1))
        echo "Waiting for ingress pods"
        ready=$(kubectl get pods -n ingress-nginx -o jsonpath='{.items[*].status.phase}' | grep -c Running || true)
        kubectl get pods -n ingress-nginx 2>&1 || true
        echo "ready: $ready"
    done
}

wait_for_nginx_pods() {
    ready=$(kubectl get pods -n kotsadm -o jsonpath='{.items[*].metadata.name} {.items[*].status.phase}' | grep "nginx" | grep -c Running || true)
    counter=0
    while [ "$ready" -lt "1" ]; do
        if [ "$counter" -gt 36 ]; then
            echo "nginx pods did not appear"
            kubectl get pods -n kotsadm -o jsonpath='{.items[*].metadata.name} {.items[*].status.phase}'
            kubectl get pods -n kotsadm
            kubectl logs -n kotsadm -l app=kotsadm
            return 1
        fi
        sleep 5
        counter=$((counter+1))
        echo "Waiting for nginx pods"
        ready=$(kubectl get pods -n kotsadm -o jsonpath='{.items[*].metadata.name} {.items[*].status.phase}' | grep "nginx" | grep -c Running || true)
        kubectl get pods -n nginx 2>&1 || true
        echo "ready: $ready"
    done
}

check_openebs_storage_class() {
    scs=$(kubectl get sc --no-headers | wc -l)
    if [ "$scs" -ne "1" ]; then
        echo "Expected 1 storage class, found $scs"
        kubectl get sc
        return 1
    fi
}

ensure_app_not_upgraded() {
    if kubectl get ns | grep -q goldpinger ; then
        echo "found goldpinger ns"
        return 1
    fi
    if kubectl get pods -n kotsadm -l app=second -q | grep -q second ; then
        echo "found pods from app update"
        return 1
    fi
}

main() {
    if ! embedded-cluster install --no-prompt --license /tmp/license.yaml 2>&1 | tee /tmp/log ; then
        cat /etc/os-release
        echo "Failed to install embedded-cluster"
        exit 1
    fi
    if ! grep -q "Admin Console is ready!" /tmp/log; then
        echo "Failed to install embedded-cluster"
        exit 1
    fi
    if ! wait_for_healthy_node; then
        echo "Failed to install embedded-cluster"
        exit 1
    fi
    if ! ensure_node_config; then
        echo "Cluster did not respect node config"
        exit 1
    fi
    if ! wait_for_pods_running 900; then
        echo "Failed to install embedded-cluster"
        exit 1
    fi
    if ! check_openebs_storage_class; then
        echo "Failed to validate if only openebs storage class is present"
        exit 1
    fi
    if ! wait_for_nginx_pods; then
        echo "Failed waiting for the application's nginx pods"
        exit 1
    fi
    if ! wait_for_ingress_pods; then
        echo "Failed waiting for ingress pods"
        exit 1
    fi
    if ! ensure_app_not_upgraded; then
        exit 1
    fi
    if ! systemctl restart embedded-cluster; then
        echo "Failed to restart embedded-cluster service"
        exit 1
    fi
}

export EMBEDDED_CLUSTER_METRICS_BASEURL="https://staging.replicated.app"
export KUBECONFIG=/root/.config/embedded-cluster/etc/kubeconfig
export PATH=$PATH:/root/.config/embedded-cluster/bin
main
