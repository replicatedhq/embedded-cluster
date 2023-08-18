#!/usr/bin/env bash
# This script is used to test the embed capability of helmvm. It will pull the memcached helm chart and
# embed it into the binary, issuing then a single node install. This script waits for the memcached pod
# to be ready.

wait_for_healthy_node() {
    ready=$(kubectl get nodes | grep -v NotReady | grep -c Ready)
    counter=0
    while [ "$ready" -lt "1" ]; do
        if [ "$counter" -gt 36 ]; then
            return 1
        fi
        sleep 5
        counter=$((counter+1))
        echo "Waiting for node to be ready"
        ready=$(kubectl get nodes | grep -v NotReady | grep -c Ready)
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
    ready=$(kubectl get pods -n helmvm | grep -v NotReady | grep Ready | grep -c memcached)
    counter=0
    while [ "$ready" -lt "1" ]; do
        if [ "$counter" -gt 36 ]; then
            return 1
        fi
        sleep 5
        counter=$((counter+1))
        echo "Waiting for memcached pods to be ready"
        ready=$(kubectl get pods -n helmvm | grep  Running | grep -c memcached)
        kubectl get pods -n helmvm 2>&1
        echo "$ready"
    done
    return 0
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
    echo "waiting for memcached " >> /tmp/log
    if ! wait_for_memcached_pods; then
        echo "Memcached pods not reporting healthy"
        exit 1
    fi
}

export KUBECONFIG=/root/.helmvm/etc/kubeconfig
export PATH=$PATH:/root/.helmvm/bin
main
