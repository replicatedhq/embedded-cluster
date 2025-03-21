#!/usr/bin/env bash
set -euox pipefail

DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
. $DIR/common.sh

function main() {
    # the version may have changed, so we need to re-install
    rm -rf "$(which kubectl-kots)"

    install_kots_cli
}

main
