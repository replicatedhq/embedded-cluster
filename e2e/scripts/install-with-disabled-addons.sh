#!/usr/bin/env bash
set -euo pipefail

wait_for_healthy_node() {
    ready=$(kubectl get nodes | grep -v NotReady | grep -c Ready || true)
    counter=0
    while [ -z "$ready" ] || [ "$ready" -lt "1" ]; do
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

check_empty_embedded_cluster_namespace() {
    pods=$(kubectl get pods -n embedded-cluster --no-headers | grep -v 'embedded-cluster-operator' | wc -l)
    if [ "$pods" -gt 0 ]; then
        kubectl get pods -n embedded-cluster
        return 1
    fi
}

check_kotsadm_namespace() {
    pods=$(kubectl get ns --no-headers | grep -c kotsadm)
    if [ "$pods" -gt 0 ]; then
        kubectl get ns
        return 1
    fi
}

main() {
    if ! embedded-cluster install --disable-addon openebs --disable-addon adminconsole --no-prompt  2>&1 | tee /tmp/log ; then
        echo "Failed to install embedded-cluster"
        exit 1
    fi
    echo "waiting for nodes" >> /tmp/log
    if ! wait_for_healthy_node; then
        echo "Nodes not reporting healthy"
        exit 1
    fi
    echo "check nothing (besides the operator) exists on embedded-cluster namespace " >> /tmp/log
    if ! check_empty_embedded_cluster_namespace; then
        echo "Pods found on embedded-cluster namespace"
        exit 1
    fi
    echo "checking if kotsadm namespace does not exist" >> /tmp/log
    if ! check_kotsadm_namespace; then
        echo "kotsadm namespace exists"
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
