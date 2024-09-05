#!/bin/bash

# shellcheck source=./common.sh
source ./scripts/common.sh

set -euo pipefail

WORKDIR=${WORKDIR:-$(pwd)}
EC_VERSION=${EC_VERSION:-}

function init_vars() {
    if [ -z "${EC_VERSION:-}" ]; then
        EC_VERSION=$(git describe --tags --match='[0-9]*.[0-9]*.[0-9]*')
    fi

    require EC_VERSION "${EC_VERSION:-}"
}

function deps() {
    make melange apko
}

function local_artifact_mirror() {
    make -C local-artifact-mirror build-ttl.sh
    cp local-artifact-mirror/build/image "local-artifact-mirror/build/image-$EC_VERSION"
}

function operator() {
    make -C operator build-ttl.sh build-chart-ttl.sh \
        PACKAGE_VERSION="$EC_VERSION"
    cp operator/build/image "operator/build/image-$EC_VERSION"
    cp operator/build/chart "operator/build/chart-$EC_VERSION"
}

function main() {
    init_vars
    deps
    local_artifact_mirror
    operator
}

main "$@"
