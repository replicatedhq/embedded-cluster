#!/usr/bin/env bash
set -euo pipefail

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
        kubectl get nodes 2>&1
    done
    return 0
}

main() {
    if ! helmvm install --no-prompt 2>&1 | tee /tmp/log ; then
        cat /etc/os-release
        echo "Failed to install helmvm"
        exit 1
    fi
    if ! grep -q "You can now access your cluster" /tmp/log; then
        echo "Failed to install helmvm"
        exit 1
    fi
    if ! wait_for_healthy_node; then
        echo "Failed to install helmvm"
        exit 1
    fi
}

export KUBECONFIG=/root/.helmvm/etc/kubeconfig
export PATH=$PATH:/root/.helmvm/bin
main
