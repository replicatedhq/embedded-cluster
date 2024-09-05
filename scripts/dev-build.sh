#!/bin/bash

set -euo pipefail

# shellcheck source=./common.sh
source ./scripts/common.sh

export EC_VERSION APP_VERSION APP_CHANNEL REPLICATED_API_ORIGIN

EC_VERSION=
APP_VERSION=
APP_CHANNEL=${APP_CHANNEL:-Unstable}
RELEASE_YAML_DIR=${RELEASE_YAML_DIR:-e2e/kots-release-install}
REPLICATED_API_ORIGIN=${REPLICATED_API_ORIGIN:-https://api.staging.replicated.com/vendor}

require REPLICATED_APP "${REPLICATED_APP:-}"
require REPLICATED_API_TOKEN "${REPLICATED_API_TOKEN:-}"
require REPLICATED_API_ORIGIN "${REPLICATED_API_ORIGIN:-}"

function main() {
    if [ -z "${EC_VERSION:-}" ]; then
        EC_VERSION=$(git describe --tags --match='[0-9]*.[0-9]*.[0-9]*')
    fi
    if [ -z "${APP_VERSION:-}" ]; then
        APP_VERSION="appver-dev-$(git rev-parse --short HEAD)"
    fi

    ./scripts/ci-build-deps.sh
    ./scripts/ci-build.sh
    ./scripts/ci-embed-release.sh
    ./scripts/cache-files.sh
    ./scripts/ci-release-app.sh
}

main "$@"
