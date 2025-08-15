#!/bin/bash

set -euo pipefail

# Detect OS and use appropriate sed syntax
if [[ "$OSTYPE" == "darwin"* ]]; then
    SED_ARGS=(-i '')
else
    SED_ARGS=(-i)
fi

function update_k0s_minor_version() {
    local minor_version=$1
    local k0s_version

    k0s_version=$(gh release list --limit 100 --repo k0sproject/k0s --exclude-pre-releases --json name,isLatest | \
        jq -r "[.[] | select(.name | startswith(\"v1.$minor_version\"))] | first | .name")

    if [[ "$k0s_version" == "null" ]]; then
        echo "No k0s version found for v1.$minor_version"
        exit 1
    fi

    sed "${SED_ARGS[@]}" "/^K0S_VERSION_1_${minor_version} = .*/d" versions.mk
    sed "${SED_ARGS[@]}" "s/^# K0S Versions$/# K0S Versions\nK0S_VERSION_1_${minor_version} = ${k0s_version}/" versions.mk
}

function update_go_dependencies() {
    make go.mod
}

function generate_crd_manifests() {
    make -C kinds generate
    make -C operator manifests
}

function update_k0s_metadata() {
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

function main() {
    local minor_version=$1
    local minor_version_minus_1=$((minor_version - 1))
    local minor_version_minus_2=$((minor_version - 2))
    local minor_version_minus_3=$((minor_version - 3))

    update_k0s_minor_version "$minor_version_minus_3"
    update_k0s_minor_version "$minor_version_minus_2"
    update_k0s_minor_version "$minor_version_minus_1"
    update_k0s_minor_version "$minor_version"

    # pin to the current major.minor version
    sed "${SED_ARGS[@]}" "s/^K0S_MINOR_VERSION \?= .*$/K0S_MINOR_VERSION ?= $minor_version/" versions.mk

    # substitute images for the major.minor version minus 2
    export K0S_MINOR_VERSION="$minor_version_minus_2"
    update_go_dependencies
    generate_crd_manifests
    update_k0s_metadata

    # reset go.mod and go.sum
    git checkout -- **/go.mod **/go.sum

    # substitute images for the major.minor version minus 1
    export K0S_MINOR_VERSION="$minor_version_minus_1"
    update_go_dependencies
    generate_crd_manifests
    update_k0s_metadata

    # reset go.mod and go.sum
    git checkout -- **/go.mod **/go.sum

    # prepare the code for the current major.minor version
    export K0S_MINOR_VERSION="$minor_version"
    update_go_dependencies
    generate_crd_manifests
    update_k0s_metadata

    echo "Done"
}

main "$@"
