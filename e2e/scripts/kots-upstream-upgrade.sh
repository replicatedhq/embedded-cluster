#!/usr/bin/env bash
set -euox pipefail

main() {
    local installation_version=
    installation_version="$1"

    if [ -z "$installation_version" ]; then
        echo "upgrading from airgap bundle"
        kubectl kots upstream upgrade embedded-cluster-smoke-test-staging-app --namespace kotsadm --airgap-bundle /tmp/upgrade/release.airgap
    else
        echo "upgrading to version ${installation_version}-upgrade from online"
        kubectl kots upstream upgrade embedded-cluster-smoke-test-staging-app --namespace kotsadm --deploy-version-label="appver-${installation_version}-upgrade"
        sleep 30 # wait for the app to deploy
    fi
}

export KUBECONFIG=/var/lib/k0s/pki/admin.conf
export PATH=$PATH:/var/lib/embedded-cluster/bin
main "$@"
