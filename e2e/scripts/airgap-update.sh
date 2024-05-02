#!/usr/bin/env bash
set -euox pipefail

main() {
    echo "upgrading from airgap bundle"
    embedded-cluster update --airgap-bundle /tmp/upgrade/release.airgap
}

export KUBECONFIG=/var/lib/k0s/pki/admin.conf
export PATH=$PATH:/var/lib/embedded-cluster/bin
main "$@"
