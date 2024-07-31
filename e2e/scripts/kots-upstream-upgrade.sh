#!/usr/bin/env bash
set -euox pipefail

main() {
    local installation_version=
    installation_version="$1"
    
    echo "upgrading to version ${installation_version}-upgrade from online"
    kubectl kots upstream upgrade embedded-cluster-smoke-test-staging-app --namespace kotsadm --deploy-version-label="appver-${installation_version}-upgrade"
    sleep 30 # wait for the app version to be created
}

export KUBECONFIG=/var/lib/k0s/pki/admin.conf
export PATH=\$PATH:/var/lib/embedded-cluster/bin
main "$@"
