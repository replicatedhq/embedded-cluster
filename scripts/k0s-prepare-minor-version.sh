#!/bin/bash

set -euo pipefail

function update_go_dependencies() {
    make go.mod
}

function generate_crd_manifests() {
    make -C kinds generate
    make -C operator manifests
}

function main() {
    export K0S_MINOR_VERSION=$1

    echo "Preparing K0S $(make print-K0S_VERSION) for release"

    update_go_dependencies
    generate_crd_manifests
}

main "$@"
