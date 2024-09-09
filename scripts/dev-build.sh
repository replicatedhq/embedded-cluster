#!/bin/bash

set -euo pipefail

# shellcheck source=./common.sh
source ./scripts/common.sh

EC_VERSION=${EC_VERSION:-}
APP_VERSION=${APP_VERSION:-}
APP_CHANNEL=${APP_CHANNEL:-Dev}
CHANNEL_ID=${CHANNEL_ID:-2lhrq5LDyoX98BdxmkHtdoqMT4P}
CHANNEL_SLUG=${CHANNEL_SLUG:-dev}
RELEASE_YAML_DIR=${RELEASE_YAML_DIR:-e2e/kots-release-install}
REPLICATED_APP=${REPLICATED_APP:-embedded-cluster-smoke-test-staging-app}
REPLICATED_API_ORIGIN=${REPLICATED_API_ORIGIN:-https://api.staging.replicated.com/vendor}
CACHE_BINS=${CACHE_BINS:-1}
ARCH=${ARCH:-arm64}
USE_CHAINGUARD=${USE_CHAINGUARD:-0}

require AWS_ACCESS_KEY_ID "${AWS_ACCESS_KEY_ID:-}"
require AWS_SECRET_ACCESS_KEY "${AWS_SECRET_ACCESS_KEY:-}"

DO_RELEASE=${DO_RELEASE:-1}

if [ "$DO_RELEASE" == "1" ]; then
    require REPLICATED_APP "${REPLICATED_APP:-}"
    require REPLICATED_API_TOKEN "${REPLICATED_API_TOKEN:-}"
    require REPLICATED_API_ORIGIN "${REPLICATED_API_ORIGIN:-}"
fi

export EC_VERSION APP_VERSION APP_CHANNEL CHANNEL_ID CHANNEL_SLUG RELEASE_YAML_DIR CACHE_BINS ARCH USE_CHAINGUARD
export REPLICATED_API_ORIGIN REPLICATED_APP REPLICATED_API_TOKEN AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY

function init_vars() {
    if [ -z "${EC_VERSION}" ]; then
        EC_VERSION=$(git describe --tags --match='[0-9]*.[0-9]*.[0-9]*')
    fi
    if [ -z "${APP_VERSION}" ]; then
        APP_VERSION="appver-dev-$(git rev-parse --short HEAD)"
    fi
}

function build() {
    ./scripts/ci-build-deps.sh
    ./scripts/ci-build.sh
    ./scripts/ci-embed-release.sh
    ./scripts/ci-cache-files.sh
    if [ "$DO_RELEASE" == "1" ]; then
        ./scripts/ci-release-app.sh
    fi
}

function clean() {
    rm -rf bin build \
        local-artifact-mirror/bin local-artifact-mirror/build \
        operator/bin operator/build || true
}

function main() {
    clean
    init_vars
    build
}

main "$@"
