#!/bin/bash

# shellcheck source=./common.sh
source ./scripts/common.sh

set -euo pipefail

EC_VERSION=${EC_VERSION:-}
APP_VERSION=${APP_VERSION:-}
APP_CHANNEL=${APP_CHANNEL:-Unstable}
RELEASE_YAML_DIR=${RELEASE_YAML_DIR:-e2e/kots-release-install}
REPLICATED_API_ORIGIN=${REPLICATED_API_ORIGIN:-https://api.staging.replicated.com/vendor}

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
    if ! command -v replicated &> /dev/null; then
        fail "replicated command not found"
    fi

    rm -rf output/tmp/release
    mkdir -p output/tmp
    cp -r "$RELEASE_YAML_DIR" output/tmp/release

    sed -i.bak "s/__version_string__/${EC_VERSION}/g" output/tmp/release/cluster-config.yaml
    replicated release create --yaml-dir output/tmp/release --promote "${APP_CHANNEL}" --version "${APP_VERSION}"
}

function main() {
    init_vars
    create_release
}

main "$@"
