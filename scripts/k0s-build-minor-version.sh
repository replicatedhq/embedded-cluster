#!/bin/bash

set -euo pipefail

function update_go_dependencies() {
    make go.mod
}

function update_images() {
    # if the metadata file for the minor version does not exist, copy it from the previous minor version
    if [ ! -f "./pkg/config/static/metadata-1_${K0S_MINOR_VERSION}.yaml" ]; then
        echo "metadata-1_${K0S_MINOR_VERSION}.yaml not found, copying from metadata-1_$((K0S_MINOR_VERSION - 1)).yaml"
        cp "./pkg/config/static/metadata-1_$((K0S_MINOR_VERSION - 1)).yaml" "./pkg/config/static/metadata-1_${K0S_MINOR_VERSION}.yaml"
    fi

    make buildtools
    ./output/bin/buildtools update images \
        --image kube-proxy --image pause \
        --image calico-cni --image calico-node --image calico-kube-controllers \
        k0s
}

function generate_crd_manifests() {
    make -C kinds generate
    make -C operator manifests
}

function main() {
    export K0S_MINOR_VERSION=$1

    echo "Preparing K0S $(make print-K0S_VERSION) for release"

    update_go_dependencies
    update_images
    generate_crd_manifests
}

main "$@"
