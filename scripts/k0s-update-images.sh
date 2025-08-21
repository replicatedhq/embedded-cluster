#!/bin/bash

set -euo pipefail

UPDATE_ALL_IMAGES=${UPDATE_ALL_IMAGES:-false}

function update_go_dependencies() {
    make go.mod
}

function update_k0s_metadata() {
    # if the metadata file for the minor version does not exist, copy it from the previous minor version
    if [ ! -f "./pkg/config/static/metadata-1_${K0S_MINOR_VERSION}.yaml" ]; then
        echo "metadata-1_${K0S_MINOR_VERSION}.yaml not found, copying from metadata-1_$((K0S_MINOR_VERSION - 1)).yaml"
        cp "./pkg/config/static/metadata-1_$((K0S_MINOR_VERSION - 1)).yaml" "./pkg/config/static/metadata-1_${K0S_MINOR_VERSION}.yaml"
    fi

    make buildtools
    if [ "$UPDATE_ALL_IMAGES" = "true" ]; then
        ./output/bin/buildtools update images k0s
    else
        ./output/bin/buildtools update images \
            --image kube-proxy --image pause \
            --image calico-cni --image calico-node --image calico-kube-controllers \
            k0s
    fi
}

function main() {
    local minor_version=$1
    local minor_version_minus_1=$((minor_version - 1))
    local minor_version_minus_2=$((minor_version - 2))

    # substitute images for the major.minor version minus 2
    export K0S_MINOR_VERSION="$minor_version_minus_2"
    update_go_dependencies
    update_k0s_metadata

    # reset go.mod and go.sum
    git checkout -- **/go.mod **/go.sum

    # substitute images for the major.minor version minus 1
    export K0S_MINOR_VERSION="$minor_version_minus_1"
    update_go_dependencies
    update_k0s_metadata

    # reset go.mod and go.sum
    git checkout -- **/go.mod **/go.sum

    # substitute images for the current major.minor version
    export K0S_MINOR_VERSION="$minor_version"
    update_go_dependencies
    update_k0s_metadata

    echo "Done"
}

main "$@"
