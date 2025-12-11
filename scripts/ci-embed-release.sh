#!/bin/bash

# shellcheck source=./common.sh
source ./scripts/common.sh

set -euo pipefail

EC_VERSION=${EC_VERSION:-}
APP_VERSION=${APP_VERSION:-}
REPLICATED_APP=${REPLICATED_APP:-embedded-cluster-smoke-test-staging-app}
REPLICATED_API_ORIGIN=${REPLICATED_API_ORIGIN:-https://api.staging.replicated.com/vendor}
APP_ID=${APP_ID:-2bViecGO8EZpChcGPeW5jbWKw2B}
APP_CHANNEL_ID=${APP_CHANNEL_ID:-}
APP_CHANNEL_SLUG=${APP_CHANNEL_SLUG:-}
RELEASE_YAML_DIR=${RELEASE_YAML_DIR:-e2e/kots-release-install}
EC_BINARY=${EC_BINARY:-output/bin/embedded-cluster}
S3_BUCKET="${S3_BUCKET:-dev-embedded-cluster-bin}"

require RELEASE_YAML_DIR "${RELEASE_YAML_DIR:-}"
require EC_BINARY "${EC_BINARY:-}"
require S3_BUCKET "${S3_BUCKET:-}"

require APP_CHANNEL_ID "${APP_CHANNEL_ID:-}"
require APP_CHANNEL_SLUG "${APP_CHANNEL_SLUG:-}"

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
}

function deps() {
    make output/bin/embedded-cluster-release-builder
    
    # Install Helm if not already installed
    if ! command -v helm &> /dev/null; then
        echo "Installing Helm..."
        curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash
    fi
}

function create_release_archive() {
    local release_url="" metadata_url=""

    rm -rf output/tmp/release
    mkdir -p output/tmp
    cp -r "$RELEASE_YAML_DIR" output/tmp/release

    # get next channel sequence
    local curr_channel_sequence=
    curr_channel_sequence=$(replicated api get "/v3/app/${APP_ID}/channel/${APP_CHANNEL_ID}/releases?pageSize=1" | jq '.releases[0].channelSequence' || true)
    if [ "$curr_channel_sequence" == "null" ]; then
        curr_channel_sequence=-1 # this is a special value that means the channel has no releases yet
    fi
    local next_channel_sequence=$((curr_channel_sequence + 1))

    {
        echo "# channel release object"
        echo "channelID: \"${APP_CHANNEL_ID}\""
        echo "channelSlug: \"${APP_CHANNEL_SLUG}\""
        echo "channelSequence: ${next_channel_sequence}"
        echo "appSlug: \"${REPLICATED_APP}\""
        echo "versionLabel: \"${APP_VERSION}\""
        echo "defaultDomains:"
        echo "  replicatedAppDomain: \"staging.replicated.app\""
        echo "  proxyRegistryDomain: \"proxy.staging.replicated.com\""
        echo "  replicatedRegistryDomain: \"registry.staging.replicated.com\""
    } > output/tmp/release/release.yaml

    if uses_dev_bucket "${S3_BUCKET:-}"; then
        release_url="https://$S3_BUCKET.s3.amazonaws.com/releases/v$(url_encode_semver "${EC_VERSION#v}").tgz"
        metadata_url="https://$S3_BUCKET.s3.amazonaws.com/metadata/v$(url_encode_semver "${EC_VERSION#v}").json"
    fi

    sed -i.bak "s|__version_string__|${EC_VERSION}|g" output/tmp/release/cluster-config.yaml
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
