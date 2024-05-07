#!/usr/bin/env bash
set -euox pipefail

main() {
    if ! embedded-cluster reset --no-prompt | tee /tmp/log ; then
        echo "Failed to uninstall embedded-cluster"
        exit 1
    fi

    if systemctl status embedded-cluster; then
        echo "Unexpectedly got status of embedded-cluster service"
        exit 1
    fi
}

export EMBEDDED_CLUSTER_METRICS_BASEURL="https://staging.replicated.app"
export KUBECONFIG=/var/lib/k0s/pki/admin.conf
export PATH=$PATH:/var/lib/embedded-cluster/bin
main
