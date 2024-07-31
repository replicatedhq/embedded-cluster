#!/usr/bin/env bash
set -euox pipefail

main() {
    local additional_flags=("$@")

    if ! embedded-cluster reset --no-prompt "${additional_flags[@]}" | tee /tmp/log ; then
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
builtin alias kubectl='k0s kubectl'

main "$@"
