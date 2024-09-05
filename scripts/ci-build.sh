#!/bin/bash

# shellcheck source=./common.sh
source ./scripts/common.sh

set -euo pipefail

WORKDIR=${WORKDIR:-$(pwd)}
EC_VERSION=${EC_VERSION:-}
K0S_VERSION=${K0S_VERSION:-}
S3_BUCKET="${S3_BUCKET:-dev-embedded-cluster-bin}"

require S3_BUCKET "${S3_BUCKET:-}"

function init_vars() {
    if [ -z "${EC_VERSION:-}" ]; then
        EC_VERSION=$(git describe --tags --match='[0-9]*.[0-9]*.[0-9]*')
    fi
    if [ -z "${K0S_VERSION:-}" ]; then
        K0S_VERSION=$(make print-K0S_VERSION)
    fi

    require EC_VERSION "${EC_VERSION:-}"
    require K0S_VERSION "${K0S_VERSION:-}"
}

function deps() {
    make buildtools
}

function binary() {
    local local_artifact_mirror_image="" k0s_binary_url="" kots_binary_url="" operator_binary_url=""

    if [ ! -f "local-artifact-mirror/build/image-$EC_VERSION" ]; then
        fail "file local-artifact-mirror/build/image-$EC_VERSION not found"
    fi

    k0s_binary_url="https://$S3_BUCKET.s3.amazonaws.com/k0s-binaries/$K0S_VERSION"
    kots_binary_url="https://$S3_BUCKET.s3.amazonaws.com/kots-binaries/$(make print-KOTS_VERSION)"
    operator_binary_url="https://$S3_BUCKET.s3.amazonaws.com/operator-binaries/$EC_VERSION"
    local_artifact_mirror_image="proxy.replicated.com/anonymous/$(cat local-artifact-mirror/build/image)"

    make -B embedded-cluster-linux-amd64 \
        K0S_VERSION="$K0S_VERSION" \
        VERSION="$EC_VERSION" \
        METADATA_K0S_BINARY_URL_OVERRIDE="$k0s_binary_url" \
        METADATA_KOTS_BINARY_URL_OVERRIDE="$kots_binary_url" \
        METADATA_OPERATOR_BINARY_URL_OVERRIDE="$operator_binary_url" \
        LOCAL_ARTIFACT_MIRROR_IMAGE="$local_artifact_mirror_image"
}

function update_operator_metadata() {
    local operator_chart=
    local operator_image=

    if [ ! -f "operator/build/image-$EC_VERSION" ]; then
        fail "file operator/build/image-$EC_VERSION not found"
    fi
    if [ ! -f "operator/build/chart-$EC_VERSION" ]; then
        fail "file operator/build/chart-$EC_VERSION not found"
    fi

    operator_chart=$(cat "operator/build/chart-$EC_VERSION")
    operator_image=$(cat "operator/build/image-$EC_VERSION")

    INPUT_OPERATOR_CHART_URL=$(echo "$operator_chart" | rev | cut -d':' -f2- | rev)
    if ! echo "$INPUT_OPERATOR_CHART_URL" | grep -q "oci://" ; then
        INPUT_OPERATOR_CHART_URL="oci://$INPUT_OPERATOR_CHART_URL"
    fi
    INPUT_OPERATOR_CHART_VERSION=$(echo "$operator_chart" | rev | cut -d':' -f1 | rev)
    INPUT_OPERATOR_IMAGE=$(echo "$operator_image" | cut -d':' -f1)

    export IMAGES_REGISTRY_SERVER=ttl.sh
    export INPUT_OPERATOR_CHART_URL
    export INPUT_OPERATOR_CHART_VERSION
    export INPUT_OPERATOR_IMAGE
    ./output/bin/buildtools update addon embeddedclusteroperator
}

function archive() {
    tar -C output/bin -czvf "$WORKDIR/embedded-cluster-linux-amd64.tgz" embedded-cluster
    log "created $WORKDIR/embedded-cluster-linux-amd64.tgz"
}

function metadata() {
    docker run --rm --platform linux/amd64 -v "$(pwd)/output/bin:/wrk" -w /wrk debian:bookworm-slim \
        ./embedded-cluster version metadata > "$WORKDIR/metadata.json"
    log "created $WORKDIR/metadata.json"
}

function main() {
    init_vars
    deps
    update_operator_metadata
    binary
    archive
    metadata
}

main "$@"
