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

    echo "installing kots cli"
    local ec_version=
    ec_version=$(embedded-cluster version | grep AdminConsole | awk '{print substr($4,2)}' | cut -d'-' -f1)
    curl "https://kots.io/install/$ec_version" | bash

}

main() {
    export HTTP_PROXY=$1
    export HTTPS_PROXY=$HTTP_PROXY
    export http_proxy=$HTTP_PROXY
    export https_proxy=$HTTP_PROXY

    install_kots_cli

    unset HTTP_PROXY
    unset HTTPS_PROXY
    unset http_proxy
    unset https_proxy
}

main "$@"
