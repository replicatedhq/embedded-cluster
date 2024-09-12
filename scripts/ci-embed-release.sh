#!/bin/bash

# shellcheck source=./common.sh
source ./scripts/common.sh

set -euo pipefail

EC_VERSION=${EC_VERSION:-}
APP_VERSION=${APP_VERSION:-}
RELEASE_YAML_DIR=${RELEASE_YAML_DIR:-e2e/kots-release-install}
EC_BINARY=${EC_BINARY:-output/bin/embedded-cluster}
S3_BUCKET="${S3_BUCKET:-dev-embedded-cluster-bin}"
USES_DEV_BUCKET=${USES_DEV_BUCKET:-1}

require RELEASE_YAML_DIR "${RELEASE_YAML_DIR:-}"
require EC_BINARY "${EC_BINARY:-}"
if [ "$USES_DEV_BUCKET" == "1" ]; then
    require S3_BUCKET "${S3_BUCKET:-}"
fi

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

function create_release_archive() {
    local release_url metadata_url

    rm -rf output/tmp/release
    mkdir -p output/tmp
    cp -r "$RELEASE_YAML_DIR" output/tmp/release

    {
        echo "# channel release object"
        echo 'channelID: "2cHXb1RCttzpR0xvnNWyaZCgDBP"'
        echo 'channelSlug: "ci"'
        echo 'appSlug: "embedded-cluster-smoke-test-staging-app"'
        echo "versionLabel: \"${APP_VERSION}\""
    } > output/tmp/release/release.yaml

    if [ "$USES_DEV_BUCKET" == "1" ]; then
        release_url="https://$S3_BUCKET.s3.amazonaws.com/releases/v$(url_encode_semver "${EC_VERSION#v}").tgz"
        metadata_url="https://$S3_BUCKET.s3.amazonaws.com/metadata/v$(url_encode_semver "${EC_VERSION#v}").json"
    fi

    sed -i.bak "s|__version_string__|${EC_VERSION}|g" output/tmp/release/cluster-config.yaml
    sed -i.bak "s|__release_url__|$release_url|g" output/tmp/release/cluster-config.yaml
    sed -i.bak "s|__metadata_url__|$metadata_url|g" output/tmp/release/cluster-config.yaml

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
