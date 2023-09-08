#!/usr/bin/env bash
# This script is used to test the embed capability of helmvm. It will pull the memcached helm chart and
# embed it into the binary, issuing then a single node install. This script waits for the memcached pod
# to be ready.
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

install_helm() {
    apt update -y
    if ! apt install -y curl ; then
        return 1
    fi
    if ! curl -fsSL -o get_helm.sh https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 ; then
        return 1
    fi
    chmod 700 get_helm.sh
    if ! ./get_helm.sh ; then
        return 1
    fi
    return 0
}

pull_helm_chart() {
    mkdir chart
    if ! helm pull oci://registry-1.docker.io/bitnamicharts/memcached --destination chart; then
        return 1
    fi
    return 0
}

embed_helm_chart() {
    fpath=$(ls -d chart/*)
    if ! helmvm embed --chart "$fpath" --output helmvm; then
        echo "Failed embed helm chart"
        exit 1
    fi
    mv helmvm /usr/local/bin
    return 0
}

wait_for_memcached_pods() {
    ready=$(kubectl get pods -n helmvm | grep -c memcached || true)
    counter=0
    while [ "$ready" -lt "1" ]; do
        if [ "$counter" -gt 36 ]; then
            return 1
        fi
        sleep 5
        counter=$((counter+1))
        echo "Waiting for memcached pods"
        ready=$(kubectl get pods -n helmvm | grep -c memcached || true)
        kubectl get pods -n helmvm 2>&1 || true
        echo "$ready"
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

main() {
    if ! install_helm ; then
        echo "Failed to install helm"
        exit 1
    fi
    if ! pull_helm_chart ; then
        echo "Failed to pull helm chart"
        exit 1
    fi
    if ! embed_helm_chart ; then
        echo "Failed to embed helm chart"
        exit 1
    fi
    if ! helmvm install --no-prompt 2>&1 | tee /tmp/log ; then
        echo "Failed to install helmvm"
        exit 1
    fi
    if ! grep -q "You can now access your cluster" /tmp/log; then
        echo "Failed to install helmvm"
        exit 1
    fi
    echo "waiting for nodes" >> /tmp/log
    if ! wait_for_healthy_node; then
        echo "Nodes not reporting healthy"
        exit 1
    fi
    if ! wait_for_pods_running 900; then
        echo "Pods not running"
        exit 1
    fi
    echo "waiting for memcached " >> /tmp/log
    if ! wait_for_memcached_pods; then
        echo "Memcached pods not present"
        exit 1
    fi
}

export KUBECONFIG=/root/.helmvm/etc/kubeconfig
export PATH=$PATH:/root/.helmvm/bin
main
