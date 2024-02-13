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
        apt-get install jq -y

        cat metadata.json

        # TODO actually edit metadata

        retry 3 aws s3 cp metadata.json "s3://${S3_BUCKET}/metadata/${EC_VERSION}.json"
    else
        echo "metadata.json not found, skipping upload"
    fi

}

function main() {
    export EC_VERSION="${EC_VERSION}-upgrade"
    metadata
}

main "$@"
