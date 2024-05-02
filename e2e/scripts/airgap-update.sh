#!/usr/bin/env bash
set -euox pipefail

main() {
    echo "upgrading from airgap bundle"
    if ! embedded-cluster update --airgap-bundle /tmp/upgrade/release.airgap; then
        echo "Failed to update embedded-cluster"
        exit 1
    fi
}

export KUBECONFIG=/var/lib/k0s/pki/admin.conf
export PATH=$PATH:/var/lib/embedded-cluster/bin
main "$@"
