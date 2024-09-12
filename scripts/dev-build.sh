#!/bin/bash

set -euo pipefail

# shellcheck source=./common.sh
source ./scripts/common.sh

EC_VERSION=${EC_VERSION:-}
APP_VERSION=${APP_VERSION:-}
APP_CHANNEL=${APP_CHANNEL:-Unstable}
RELEASE_YAML_DIR=${RELEASE_YAML_DIR:-e2e/kots-release-install}
REPLICATED_API_ORIGIN=${REPLICATED_API_ORIGIN:-https://api.staging.replicated.com/vendor}

SKIP_S3_UPLOAD=${SKIP_S3_UPLOAD:-0}

ARCH=${ARCH:-amd64}
USE_CHAINGUARD=${USE_CHAINGUARD:-0}
S3_BUCKET="${S3_BUCKET:-dev-embedded-cluster-bin}"
USES_DEV_BUCKET=${USES_DEV_BUCKET:-1}

require AWS_ACCESS_KEY_ID "${AWS_ACCESS_KEY_ID:-}"
require AWS_SECRET_ACCESS_KEY "${AWS_SECRET_ACCESS_KEY:-}"

SKIP_RELEASE=${SKIP_RELEASE:-0}

if [ "$SKIP_RELEASE" != "1" ]; then
    require REPLICATED_APP "${REPLICATED_APP:-}"
    require REPLICATED_API_TOKEN "${REPLICATED_API_TOKEN:-}"
    require REPLICATED_API_ORIGIN "${REPLICATED_API_ORIGIN:-}"
fi

export EC_VERSION APP_VERSION APP_CHANNEL RELEASE_YAML_DIR ARCH USE_CHAINGUARD USES_DEV_BUCKET
export REPLICATED_API_ORIGIN REPLICATED_APP REPLICATED_API_TOKEN AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY

function init_vars() {
    if [ -z "${EC_VERSION:-}" ]; then
        EC_VERSION=$(git describe --tags --match='[0-9]*.[0-9]*.[0-9]*')
    fi
    if [ -z "${APP_VERSION:-}" ]; then
        APP_VERSION="appver-dev-$(git rev-parse --short HEAD)"
    fi
}

function build() {
    ./scripts/ci-build-deps.sh
    ./scripts/ci-build.sh
    ./scripts/ci-embed-release.sh
    if [ "$SKIP_S3_UPLOAD" != "1" ]; then
        ./scripts/ci-cache-files.sh
    fi
    if [ "$SKIP_RELEASE" != "1" ]; then
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
