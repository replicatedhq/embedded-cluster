#!/bin/bash

main() {
    nodes=$1
    export KUBECONFIG=/root/.helmvm/etc/kubeconfig
    export PATH=$PATH:/root/.helmvm/bin
    kubectl get nodes
    ready=$(kubectl get nodes | grep Ready | grep -v NotReady | wc -l)
    while [ $ready -lt $nodes ]; do
        echo "Waiting for $nodes nodes to be ready, currently $ready"
        sleep 5
        ready=$(kubectl get nodes | grep Ready | grep -v NotReady | wc -l)
        kubectl get nodes
    done
}

main "$@"
