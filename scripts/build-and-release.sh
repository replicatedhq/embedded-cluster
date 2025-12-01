#!/bin/bash

set -euo pipefail

# shellcheck source=./common.sh
source ./scripts/common.sh

EC_VERSION=${EC_VERSION:-}
APP_VERSION=${APP_VERSION:-}
APP_ID=${APP_ID:-2bViecGO8EZpChcGPeW5jbWKw2B}
APP_CHANNEL=${APP_CHANNEL:-}
RELEASE_YAML_DIR=${RELEASE_YAML_DIR:-e2e/kots-release-install}
REPLICATED_APP=${REPLICATED_APP:-embedded-cluster-smoke-test-staging-app}
REPLICATED_API_ORIGIN=${REPLICATED_API_ORIGIN:-https://api.staging.replicated.com/vendor}
UPLOAD_BINARIES=${UPLOAD_BINARIES:-1}
ARCH=${ARCH:-$(go env GOARCH)}
USE_CHAINGUARD=${USE_CHAINGUARD:-0}
S3_BUCKET="${S3_BUCKET:-tf-staging-embedded-cluster-bin}"

USES_DEV_BUCKET=0
if [ "$S3_BUCKET" != "tf-staging-embedded-cluster-bin" ]; then
    USES_DEV_BUCKET=1
fi

ensure_secret "AWS_ACCESS_KEY_ID" "ARTIFACT_UPLOAD_AWS_ACCESS_KEY_ID"
ensure_secret "AWS_SECRET_ACCESS_KEY" "ARTIFACT_UPLOAD_AWS_SECRET_ACCESS_KEY"

SKIP_RELEASE=${SKIP_RELEASE:-0}

if [ "$SKIP_RELEASE" != "1" ]; then
    require REPLICATED_APP "${REPLICATED_APP:-}"
    ensure_secret "REPLICATED_API_TOKEN" "STAGING_REPLICATED_API_TOKEN"
    require REPLICATED_API_ORIGIN "${REPLICATED_API_ORIGIN:-}"
fi

export EC_VERSION APP_VERSION APP_ID APP_CHANNEL APP_CHANNEL_ID APP_CHANNEL_SLUG RELEASE_YAML_DIR UPLOAD_BINARIES ARCH USE_CHAINGUARD USES_DEV_BUCKET
export REPLICATED_API_ORIGIN REPLICATED_APP REPLICATED_API_TOKEN AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY

function init_vars() {
    if [ -z "${EC_VERSION:-}" ]; then
        EC_VERSION=$(git describe --tags --match='[0-9]*.[0-9]*.[0-9]*' --abbrev=4)
    fi
    if [ -z "${APP_VERSION:-}" ]; then
        APP_VERSION="appver-dev-$(git rev-parse --short HEAD)"
    fi
    # if the IMAGES_REGISTRY_SERVER environment variable is set, print a warning, ths can cause build failures
    if [ -n "${IMAGES_REGISTRY_SERVER:-}" ]; then
        echo "WARNING: IMAGES_REGISTRY_SERVER is set, this can cause build failures"
    fi

    ensure_local_dev_env
}

function build() {
    ./scripts/ci-build-deps.sh
    ./scripts/ci-build-bin.sh
    ./scripts/ci-embed-release.sh
    ./scripts/ci-upload-binaries.sh
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
