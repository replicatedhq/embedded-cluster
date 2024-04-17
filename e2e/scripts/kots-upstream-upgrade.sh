#!/usr/bin/env bash
set -euox pipefail


maybe_install_curl() {
    if ! command -v curl; then
        apt-get update
        apt-get install -y curl
    fi
}

install_kots_cli() {
    maybe_install_curl

    # install kots CLI
    echo "installing kots cli"
    local ec_version=
    ec_version=$(embedded-cluster version | grep AdminConsole | awk '{print substr($4,2)}' | cut -d'-' -f1)
    curl "https://kots.io/install/$ec_version" | bash

}


main() {
    local installation_version=
    installation_version="$1"

    export HTTP_PROXY=http://10.0.0.254:3128
    export HTTPS_PROXY=$HTTP_PROXY

    install_kots_cli

    unset HTTP_PROXY
    unset HTTPS_PROXY

    echo "upgrading to version ${installation_version}-upgrade"
    kubectl kots upstream upgrade embedded-cluster-smoke-test-staging-app --namespace kotsadm --airgap-bundle /tmp/upgrade/release.airgap
}

export KUBECONFIG=/var/lib/k0s/pki/admin.conf
export PATH=$PATH:/var/lib/embedded-cluster/bin
main "$@"
