#!/usr/bin/env bash
set -euox pipefail

main() {
    local installation_version=
    installation_version="$1"

    echo "upgrading to version ${installation_version}-upgrade"
    kubectl kots upstream upgrade embedded-cluster-smoke-test-staging-app --namespace kotsadm --airgap-bundle /tmp/upgrade/release.airgap
}

export KUBECONFIG=/var/lib/k0s/pki/admin.conf
export PATH=$PATH:/var/lib/embedded-cluster/bin
main "$@"
