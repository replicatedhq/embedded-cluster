#!/bin/bash

# shellcheck source=./common.sh
source ./scripts/common.sh

set -euxo pipefail

EC_VERSION=${EC_VERSION:-}
APP_VERSION=${APP_VERSION:-}
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
}

function deps() {
    make output/bin/embedded-cluster-release-builder
}

function create_release() {
    rm -rf output/tmp
    mkdir -p output/tmp
    cp -r e2e/kots-release-install output/tmp

    {
        echo "# channel release object"
        echo 'channelID: "2cHXb1RCttzpR0xvnNWyaZCgDBP"'
        echo 'channelSlug: "ci"'
        echo 'appSlug: "embedded-cluster-smoke-test-staging-app"'
        echo "versionLabel: \"${APP_VERSION}\""
    } > output/tmp/kots-release-install/release.yaml

    # ensure that the cluster config embedded in the CI binaries is correct
    sed -i .bak "s/__version_string__/${EC_VERSION//./\\.}/g" output/tmp/kots-release-install/cluster-config.yaml

    tar -czf output/tmp/release.tar.gz -C output/tmp/kots-release-install .

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
    create_release
    build
}

main "$@"
