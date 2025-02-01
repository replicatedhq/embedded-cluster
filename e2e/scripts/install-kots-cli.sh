#!/usr/bin/env bash
set -euo pipefail

function main() {
    # the version may have changed, so we need to re-install
    rm -rf "$(which kubectl-kots)"

    install_kots_cli
}

main
