#!/usr/bin/env bash
set -euo pipefail

main() {
    kubectl describe pods -n registry
    kubectl logs deploy/registry -n registry
    kubectl logs deploy/registry -n registry -p
}

export EMBEDDED_CLUSTER_METRICS_BASEURL="https://staging.replicated.app"
export KUBECONFIG=/var/lib/k0s/pki/admin.conf
export PATH=$PATH:/var/lib/embedded-cluster/bin
main "$@"
