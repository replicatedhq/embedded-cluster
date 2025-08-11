#!/bin/bash

set -euo pipefail

function main() {
    local minor_version=$1
    local previous_minor_version=$((minor_version - 1))
    local k0s_version previous_k0s_version

    k0s_version=$(gh release list --repo k0sproject/k0s --exclude-pre-releases --json name | \
        jq -r "[.[] | select(.name | startswith(\"v1.$minor_version\"))] | first | .name")

    previous_k0s_version=$(gh release list --repo k0sproject/k0s --exclude-pre-releases --json name | \
        jq -r "[.[] | select(.name | startswith(\"v1.$previous_minor_version\"))] | first | .name")

    sed -i '' "/^K0S_VERSION/s/=.*/= $k0s_version/" Makefile
    sed -i '' "/^K0S_GO_VERSION/s/=.*/= $k0s_version/" Makefile

    sed -i '' "/^PREVIOUS_K0S_VERSION/s/=.*/= $previous_k0s_version/" Makefile
    sed -i '' "/^PREVIOUS_K0S_GO_VERSION/s/=.*/= $previous_k0s_version/" Makefile

    echo "Preparing K0S $(make print-K0S_VERSION) for release"

    make go.mod

    make buildtools
    ./output/bin/buildtools update images --image kube-proxy --image pause k0s

    make -C kinds generate
    make -C operator manifests

    echo "Done"
}

main "$@"
