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

    # update images for all major.minor versions
    ./scripts/k0s-update-images.sh "$minor_version"

    # prepare the code for the current major.minor version
    export K0S_MINOR_VERSION="$minor_version"
    update_go_dependencies
    generate_crd_manifests

    echo "Done"
}

main "$@"
