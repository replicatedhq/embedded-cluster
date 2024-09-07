#!/bin/bash

# shellcheck source=./common.sh
source ./scripts/common.sh

set -euo pipefail

EC_VERSION=${EC_VERSION:-}
APP_VERSION=${APP_VERSION:-}
APP_SLUG=${APP_SLUG:-embedded-cluster-smoke-test-staging-app}
CHANNEL_ID=${CHANNEL_ID:-2lhrq5LDyoX98BdxmkHtdoqMT4P}
CHANNEL_SLUG=${CHANNEL_SLUG:-dev}
RELEASE_YAML_DIR=${RELEASE_YAML_DIR:-e2e/kots-release-install}
EC_BINARY=${EC_BINARY:-output/bin/embedded-cluster}

function init_vars() {
    if [ -z "${EC_VERSION:-}" ]; then
        EC_VERSION=$(git describe --tags --match='[0-9]*.[0-9]*.[0-9]*')
    fi
    if [ -z "${APP_VERSION:-}" ]; then
        local short_sha=
        short_sha=$(git rev-parse --short HEAD)
        if [ -z "$short_sha" ]; then
            fail "unable to get short sha"
        fi
        APP_VERSION="appver-dev-$short_sha"
    fi

    require EC_VERSION "${EC_VERSION:-}"
    require APP_VERSION "${APP_VERSION:-}"
    require RELEASE_YAML_DIR "${RELEASE_YAML_DIR:-}"
    require EC_BINARY "${EC_BINARY:-}"
}

function deps() {
    make output/bin/embedded-cluster-release-builder
}

function create_release_archive() {
    rm -rf output/tmp/release
    mkdir -p output/tmp
    cp -r "$RELEASE_YAML_DIR" output/tmp/release

    {
        echo "# channel release object"
        echo "channelID: \"${CHANNEL_ID}\""
        echo "channelSlug: \"${CHANNEL_SLUG}\""
        echo "appSlug: \"${APP_SLUG}\""
        echo "versionLabel: \"${APP_VERSION}\""
    } > output/tmp/release/release.yaml

    # ensure that the cluster config embedded in the CI binaries is correct
    sed -i.bak "s|__version_string__|${EC_VERSION}|g" output/tmp/release/cluster-config.yaml
    # remove the release and metadata override urls
    sed -i.bak "s|__release_url__||g" output/tmp/release/cluster-config.yaml
    sed -i.bak "s|__metadata_url__||g" output/tmp/release/cluster-config.yaml

    tar -czf output/tmp/release.tar.gz -C output/tmp/release .

    log "created output/tmp/release.tar.gz"
}

function build() {
    if [ ! -f "$EC_BINARY" ]; then
        fail "file $EC_BINARY not found"
    fi

    ./output/bin/embedded-cluster-release-builder \
        "$EC_BINARY" output/tmp/release.tar.gz "$EC_BINARY"
}

function main() {
    init_vars
    deps
    create_release_archive
    build
}

main "$@"
