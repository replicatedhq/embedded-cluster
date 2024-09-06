#!/bin/bash

# shellcheck source=./common.sh
source ./scripts/common.sh

set -euo pipefail

EC_VERSION=${EC_VERSION:-}
APP_VERSION=${APP_VERSION:-}
APP_CHANNEL=${APP_CHANNEL:-Dev}
RELEASE_YAML_DIR=${RELEASE_YAML_DIR:-e2e/kots-release-install}
REPLICATED_APP=${REPLICATED_APP:-embedded-cluster-smoke-test-staging-app}
REPLICATED_API_ORIGIN=${REPLICATED_API_ORIGIN:-https://api.staging.replicated.com/vendor}
S3_BUCKET="${S3_BUCKET:-dev-embedded-cluster-bin}"

require S3_BUCKET "${S3_BUCKET:-}"
require REPLICATED_APP "${REPLICATED_APP:-}"
require REPLICATED_API_TOKEN "${REPLICATED_API_TOKEN:-}"
require REPLICATED_API_ORIGIN "${REPLICATED_API_ORIGIN:-}"

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
    require APP_CHANNEL "${APP_CHANNEL:-}"
    require RELEASE_YAML_DIR "${RELEASE_YAML_DIR:-}"
}

function ensure_app_channel() {
    if ! replicated app channels list | grep -q "${APP_CHANNEL}"; then
        fail "app channel ${APP_CHANNEL} not found"
    fi
}

function create_release() {
    local release_url metadata_url

    if ! command -v replicated &> /dev/null; then
        fail "replicated command not found"
    fi

    rm -rf output/tmp/release
    mkdir -p output/tmp
    cp -r "$RELEASE_YAML_DIR" output/tmp/release

    release_url="https://$S3_BUCKET.s3.amazonaws.com/releases/v$(url_encode_semver "${EC_VERSION#v}").tgz"
    metadata_url="https://$S3_BUCKET.s3.amazonaws.com/metadata/v$(url_encode_semver "${EC_VERSION#v}").json"

    sed -i.bak "s|__version_string__|${EC_VERSION}|g" output/tmp/release/cluster-config.yaml
    sed -i.bak "s|__release_url__|$release_url|g" output/tmp/release/cluster-config.yaml
    sed -i.bak "s|__metadata_url__|$metadata_url|g" output/tmp/release/cluster-config.yaml

    export REPLICATED_APP REPLICATED_API_TOKEN REPLICATED_API_ORIGIN
    replicated release create --yaml-dir output/tmp/release --promote "${APP_CHANNEL}" --version "${APP_VERSION}"
}

function main() {
    init_vars
    create_release
}

main "$@"
