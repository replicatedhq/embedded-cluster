#!/bin/bash

set -eo pipefail

# shellcheck source=./common.sh
source ./scripts/common.sh

require AWS_ACCESS_KEY_ID "${AWS_ACCESS_KEY_ID}"
require AWS_SECRET_ACCESS_KEY "${AWS_SECRET_ACCESS_KEY}"
require AWS_REGION "${AWS_REGION}"
require S3_BUCKET "${S3_BUCKET}"

function metadata() {
    if [ -z "${EC_VERSION}" ]; then
        echo "EC_VERSION unset, not uploading metadata-previous-k0s.json"
        return 0
    fi

    # append a 'v' prefix to the version if it doesn't already have one
    local version="$EC_VERSION"
    if ! echo "$version" | grep -q "^v"; then
        version="v$version"
    fi

    # mutate the metadata-previous-k0s.json to create a suitable upgrade
    if [ -f metadata-previous-k0s.json ]; then
        sudo apt-get install jq -y

        jq '(.Configs.charts[] | select(.name == "embedded-cluster-operator")).values += "resources:\n  requests:\n    cpu: 123m"' metadata-previous-k0s.json > install-metadata.json
        cat install-metadata.json

        retry 3 aws s3 cp --no-progress install-metadata.json "s3://${S3_BUCKET}/metadata/${version}.json"
    else
        echo "metadata-previous-k0s.json not found, skipping upload"
    fi

}

function embeddedcluster() {
    if [ -z "${EC_VERSION}" ]; then
        echo "EC_VERSION unset, not uploading embedded cluster release"
        return 0
    fi

    # append a 'v' prefix to the version if it doesn't already have one
    local version="$EC_VERSION"
    if ! echo "$version" | grep -q "^v"; then
        version="v$version"
    fi

    # check if a file 'embedded-cluster-linux-amd64-previous-k0s.tgz' exists in the directory
    # if it does, upload it as releases/${version}.tgz
    if [ -f embedded-cluster-linux-amd64-previous-k0s.tgz ]; then
        retry 3 aws s3 cp --no-progress embedded-cluster-linux-amd64-previous-k0s.tgz "s3://${S3_BUCKET}/releases/${version}.tgz"
    else
        echo "embedded-cluster-linux-amd64-previous-k0s.tgz not found, skipping upload"
    fi
}

function main() {
    export EC_VERSION="${EC_VERSION}-previous-k0s"
    metadata
    embeddedcluster
}

main "$@"
