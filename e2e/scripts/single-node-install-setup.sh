#!/usr/bin/env bash
set -euxo pipefail

DIR=/usr/local/bin
. $DIR/common.sh

function main() {
    if ! install_kots_cli; then
        echo "Failed to install kots cli"
        exit 1
    fi
}

main "$@"
