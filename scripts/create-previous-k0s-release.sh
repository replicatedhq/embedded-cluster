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

    # mutate the build/metadata-previous-k0s.json to create a suitable upgrade
    if [ -f build/metadata-previous-k0s.json ]; then
        sudo apt-get install jq -y

        jq '(.Configs.charts[] | select(.name == "embedded-cluster-operator")).values += "resources:\n  requests:\n    cpu: 123m"' build/metadata-previous-k0s.json > build/install-metadata.json
        cat build/install-metadata.json

        # append a 'v' prefix to the version if it doesn't already have one
        retry 3 aws s3 cp --no-progress build/install-metadata.json "s3://${S3_BUCKET}/metadata/v${EC_VERSION#v}.json"
    else
        echo "build/metadata-previous-k0s.json not found, skipping upload"
    fi

}

function embeddedcluster() {
    if [ -z "${EC_VERSION}" ]; then
        echo "EC_VERSION unset, not uploading embedded cluster release"
        return 0
    fi

    # check if a file 'build/embedded-cluster-linux-$ARCH-previous-k0s.tgz' exists in the directory
    # if it does, upload it as releases/v${EC_VERSION}.tgz
    if [ -f "build/embedded-cluster-linux-$ARCH-previous-k0s.tgz" ]; then
        # append a 'v' prefix to the version if it doesn't already have one
        retry 3 aws s3 cp --no-progress build/embedded-cluster-linux-$ARCH-previous-k0s.tgz "s3://${S3_BUCKET}/releases/v${EC_VERSION#v}.tgz"
    else
        echo "build/embedded-cluster-linux-$ARCH-previous-k0s.tgz not found, skipping upload"
    fi
}

function main() {
    export EC_VERSION="${EC_VERSION}-previous-k0s"
    metadata
    embeddedcluster
}

main "$@"
