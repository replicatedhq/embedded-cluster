#!/bin/bash

set -euo pipefail

function require() {
    if [ -z "$2" ]; then
        echo "validation failed: $1 unset"
        exit 1
    else
        echo "$1 is set to $2"
    fi
}

require EC_VERSION "${EC_VERSION}"
require K0S_VERSION "${K0S_VERSION}"
require LOCAL_ARTIFACT_MIRROR_IMAGE "${LOCAL_ARTIFACT_MIRROR_IMAGE}"

WORKDIR=

function workdir() {
    if [ -z "${WORKDIR:-}" ]; then
        WORKDIR="$(pwd)"
    fi
}

function build() {
    make -B embedded-cluster-linux-amd64 \
        K0S_VERSION="$K0S_VERSION" \
        VERSION="$EC_VERSION"
}

function archive() {
    tar -C output/bin -czvf "$WORKDIR/embedded-cluster-linux-amd64.tgz" embedded-cluster
    echo "created $WORKDIR/embedded-cluster-linux-amd64.tgz"
}

function metadata() {
    docker run --rm --platform linux/amd64 -v "$(pwd)/output/bin:/wrk" -w /wrk debian:bookworm-slim \
        ./embedded-cluster version metadata > "$WORKDIR/metadata.json"
    echo "created $WORKDIR/metadata.json"
}

function main() {
    workdir
    build
    archive
    metadata
}

main "$@"
