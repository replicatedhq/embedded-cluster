#!/bin/bash

# shellcheck source=./common.sh
source ./scripts/common.sh

set -euo pipefail

EC_VERSION=${EC_VERSION:-}
USE_CHAINGUARD=${USE_CHAINGUARD:-1}

function init_vars() {
    if [ -z "${EC_VERSION:-}" ]; then
      EC_VERSION=$(git describe --tags --match='[0-9]*.[0-9]*.[0-9]*' --abbrev=4)
    fi

    require EC_VERSION "${EC_VERSION:-}"
}

function local_artifact_mirror() {
    make -C local-artifact-mirror build-ttl.sh 2>&1 | prefix_output "LAM"
    cp local-artifact-mirror/build/image "local-artifact-mirror/build/image-$EC_VERSION"
}

function operator() {
    make -C operator build-ttl.sh build-chart-ttl.sh \
        PACKAGE_VERSION="$EC_VERSION" \
        VERSION="$EC_VERSION" 2>&1 | prefix_output "OPERATOR"
    cp operator/build/image "operator/build/image-$EC_VERSION"
    cp operator/build/chart "operator/build/chart-$EC_VERSION"
}

function main() {
    init_vars
    
    local_artifact_mirror &
    lam_pid=$!
    
    operator &
    operator_pid=$!
    
    wait $lam_pid
    wait $operator_pid
}

main "$@"
