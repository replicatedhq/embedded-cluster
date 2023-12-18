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

install_helm() {
    apt-get update -y
    if ! apt-get install -y curl ; then
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
    if ! helm repo add bitnami https://charts.bitnami.com/bitnami ; then
        return 1
    fi
    if ! helm pull bitnami/memcached --version 6.6.2 --destination chart; then
        return 1
    fi
    return 0
}

embed_helm_chart() {
    fpath=$(ls -d chart/*)
    if ! embedded-cluster embed --chart "$fpath" --output embedded-cluster; then
        return 1
    fi
    mv embedded-cluster /usr/local/bin
    return 0
}

check_empty_embedded_cluster_namespace() {
    pods=$(kubectl get pods -n embedded-cluster --no-headers | grep -v 'embedded-cluster-operator' | wc -l)
    if [ "$pods" -gt 0 ]; then
        kubectl get pods -n embedded-cluster
        return 1
    fi
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
    if ! embedded-cluster install --disable-addon openebs --disable-addon adminconsole --disable-addon memcached --no-prompt  2>&1 | tee /tmp/log ; then
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
    if ! systemctl restart embedded-cluster; then
        echo "Failed to restart embedded-cluster service"
        exit 1
    fi
}

export EMBEDDED_CLUSTER_METRICS_BASEURL="https://staging.replicated.app"
export KUBECONFIG=/root/.config/embedded-cluster/etc/kubeconfig
export PATH=$PATH:/root/.config/embedded-cluster/bin
main
