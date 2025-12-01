#!/bin/bash

# shellcheck source=./common.sh
source ./scripts/common.sh

set -euo pipefail

EC_VERSION=${EC_VERSION:-}
APP_VERSION=${APP_VERSION:-}
APP_CHANNEL=${APP_CHANNEL:-}
RELEASE_YAML_DIR=${RELEASE_YAML_DIR:-e2e/kots-release-install}
REPLICATED_APP=${REPLICATED_APP:-embedded-cluster-smoke-test-staging-app}
REPLICATED_API_ORIGIN=${REPLICATED_API_ORIGIN:-https://api.staging.replicated.com/vendor}
S3_BUCKET="${S3_BUCKET:-tf-staging-embedded-cluster-bin}"
V2_ENABLED=${V2_ENABLED:-0}

require S3_BUCKET "${S3_BUCKET:-}"
require REPLICATED_APP "${REPLICATED_APP:-}"
ensure_secret "REPLICATED_API_TOKEN" "STAGING_REPLICATED_API_TOKEN"
require REPLICATED_API_ORIGIN "${REPLICATED_API_ORIGIN:-}"

require APP_CHANNEL "${APP_CHANNEL:-}"

function init_vars() {
    if [ -z "${EC_VERSION:-}" ]; then
        EC_VERSION=$(git describe --tags --match='[0-9]*.[0-9]*.[0-9]*' --abbrev=4)
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
    
    # Install Helm if not already installed
    if ! command -v helm &> /dev/null; then
        echo "Installing Helm..."
        curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash
    fi
}

function create_release() {
    local release_url="" metadata_url=""

    if ! command -v replicated &> /dev/null; then
        fail "replicated command not found"
    fi

    rm -rf output/tmp/release
    mkdir -p output/tmp
    cp -r "$RELEASE_YAML_DIR" output/tmp/release

    if uses_dev_bucket "${S3_BUCKET:-}"; then
        release_url="https://$S3_BUCKET.s3.amazonaws.com/releases/v$(url_encode_semver "${EC_VERSION#v}").tgz"
        metadata_url="https://$S3_BUCKET.s3.amazonaws.com/metadata/v$(url_encode_semver "${EC_VERSION#v}").json"
    fi

    sed -i.bak "s|__version_string__|${EC_VERSION}|g" output/tmp/release/cluster-config.yaml
    if [ "$V2_ENABLED" == "1" ]; then
        sed -i.bak "s|__v2_enabled__|true|g" output/tmp/release/cluster-config.yaml
    else
        sed -i.bak "s|__v2_enabled__|false|g" output/tmp/release/cluster-config.yaml
    fi
    sed -i.bak "s|__release_url__|$release_url|g" output/tmp/release/cluster-config.yaml
    sed -i.bak "s|__metadata_url__|$metadata_url|g" output/tmp/release/cluster-config.yaml
    
    # Clean up backup files
    find output/tmp/release -name "*.bak" -type f -delete
    
    # Package the Helm charts
    for CHART in nginx-app redis-app; do
        if [ -d "e2e/helm-charts/$CHART" ]; then
            echo "Packaging Helm chart: $CHART..."
            helm package -u e2e/helm-charts/$CHART -d output/tmp/release
            
            # Get the packaged chart filename
            CHART_FILENAME=$(find output/tmp/release -name "$CHART-*.tgz" -type f | head -1)
            if [ -n "$CHART_FILENAME" ]; then
                echo "Created Helm chart package: $CHART_FILENAME"
            else
                echo "Warning: Failed to create Helm chart package for $CHART"
            fi
        else
            echo "Helm chart directory not found at e2e/helm-charts/$CHART"
        fi
    done

    export REPLICATED_APP REPLICATED_API_TOKEN REPLICATED_API_ORIGIN
    replicated release create --yaml-dir output/tmp/release --promote "${APP_CHANNEL}" --version "${APP_VERSION}"
}

function main() {
    init_vars
    create_release
}

main "$@"
