#!/bin/bash

set -euo pipefail

# shellcheck source=./common.sh
source ./scripts/common.sh

EC_VERSION=
APP_VERSION=
APP_CHANNEL=${APP_CHANNEL:-Unstable}
RELEASE_YAML_DIR=${RELEASE_YAML_DIR:-e2e/kots-release-install}
REPLICATED_API_ORIGIN=${REPLICATED_API_ORIGIN:-https://api.staging.replicated.com/vendor}

ARCH=${ARCH:-amd64}
USE_CHAINGUARD=${USE_CHAINGUARD:-0}
DO_RELEASE=${DO_RELEASE:-1}

require REPLICATED_APP "${REPLICATED_APP:-}"
require REPLICATED_API_TOKEN "${REPLICATED_API_TOKEN:-}"
require REPLICATED_API_ORIGIN "${REPLICATED_API_ORIGIN:-}"
require AWS_ACCESS_KEY_ID "${AWS_ACCESS_KEY_ID:-}"
require AWS_SECRET_ACCESS_KEY "${AWS_SECRET_ACCESS_KEY:-}"

export EC_VERSION APP_VERSION APP_CHANNEL RELEASE_YAML_DIR ARCH USE_CHAINGUARD
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
    ./scripts/cache-files.sh
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
    init_vars
    build
    clean
}

main "$@"
