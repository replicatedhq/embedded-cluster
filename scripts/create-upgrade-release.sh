#!/bin/bash

set -eo pipefail

function require() {
    if [ -z "$2" ]; then
        echo "validation failed: $1 unset"
        exit 1
    fi
}

require AWS_ACCESS_KEY_ID "${AWS_ACCESS_KEY_ID}"
require AWS_SECRET_ACCESS_KEY "${AWS_SECRET_ACCESS_KEY}"
require AWS_REGION "${AWS_REGION}"
require S3_BUCKET "${S3_BUCKET}"

function retry() {
    local retries=$1
    shift

    local count=0
    until "$@"; do
        exit=$?
        wait=$((2 ** $count))
        count=$(($count + 1))
        if [ $count -lt $retries ]; then
            echo "Retry $count/$retries exited $exit, retrying in $wait seconds..."
            sleep $wait
        else
            echo "Retry $count/$retries exited $exit, no more retries left."
            return $exit
        fi
    done
    return 0
}

function metadata() {
    if [ -z "${EC_VERSION}" ]; then
        echo "EC_VERSION unset, not uploading metadata.json"
        return 0
    fi

    # mutate the metadata.json to create a suitable upgrade
    if [ -f metadata.json ]; then
        sudo apt-get install jq -y

        jq '(.Configs.charts[] | select(.name == "embedded-cluster-operator")).values += "resources:\n  requests:\n    cpu: 123m"' metadata-upgrade.json > upgrade-metadata.json
        cat upgrade-metadata.json

        retry 3 aws s3 cp --no-progress upgrade-metadata.json "s3://${S3_BUCKET}/metadata/${EC_VERSION}.json"
    else
        echo "metadata.json not found, skipping upload"
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

    # check if a file 'embedded-cluster-linux-amd64.tgz' exists in the directory
    # if it does, upload it as releases/${version}.tgz
    if [ -f embedded-cluster-linux-amd64.tgz ]; then
        retry 3 aws s3 cp --no-progress embedded-cluster-linux-amd64.tgz "s3://${S3_BUCKET}/releases/${version}.tgz"
    else
        echo "embedded-cluster-linux-amd64.tgz not found, skipping upload"
    fi
}

function main() {
    export EC_VERSION="${EC_VERSION}-upgrade"
    metadata
    embeddedcluster
}

main "$@"
