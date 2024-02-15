#!/usr/bin/env bash
set -euo pipefail
if ! NODE_PATH="$(npm root -g)" "$@"; then
    export EMBEDDED_CLUSTER_METRICS_BASEURL="https://staging.replicated.app"
    export KUBECONFIG=/root/.config/embedded-cluster/etc/kubeconfig
    export PATH=$PATH:/root/.config/embedded-cluster/bin
    sleep 30
    echo "pods"
    kubectl get pods -A
    echo "password secret"
    kubectl get secret -n kotsadm kotsadm-password -o yaml
fi
