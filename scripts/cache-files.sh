#!/bin/bash

set -eo pipefail

# shellcheck source=list-all-packages.sh
source ./bin/list-all-packages.sh

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

function k0sbin() {
    # first, figure out what version of k0s is in the current build
    local k0s_version=
    k0s_version=$(awk '/^K0S_VERSION/{print $3}' Makefile)
    local k0s_override=
    k0s_override=$(awk '/^K0S_BINARY_SOURCE_OVERRIDE/{print $3}' Makefile)

    # if the override is set, the binary will have been added to the bucket through another process
    if [ -n "${k0s_override}" ]; then
        echo "K0S_BINARY_SOURCE_OVERRIDE is set, skipping upload"
        exit 0
    fi

    # check if the binary already exists in the bucket
    local k0s_binary_exists=
    k0s_binary_exists=$(aws s3api head-object --bucket "${S3_BUCKET}" --key "k0s-binaries/${k0s_version}" || true)

    # if the binary already exists, we don't need to upload it again
    if [ -n "${k0s_binary_exists}" ]; then
        echo "k0s binary ${k0s_version} already exists in bucket ${S3_BUCKET}, skipping upload"
        exit 0
    fi

    # download the k0s binary from official sources
    curl -L -o "$(k0s_version)" "https://github.com/k0sproject/k0s/releases/download/$(k0s_version)/k0s-$(k0s_version)-amd64"

    # upload the binary to the bucket
    retry 3 aws s3 cp "$(k0s_version)" "s3://${S3_BUCKET}/k0s-binaries/${k0s_version}"
}

function metadata() {
    if [ -z "${EC_VERSION}" ]; then
        echo "EC_VERSION unset, not uploading metadata.json"
        exit 0
    fi

    # check if a file 'metadata.json' exists in the directory
    # if it does, upload it as metadata/${ec_version}.json
    if [ -f metadata.json ]; then
        retry 3 aws s3 cp metadata.json "s3://${S3_BUCKET}/metadata/${EC_VERSION}.json"
    else
        echo "metadata.json not found, skipping upload"
    fi

}

# there are two files to be uploaded for each release - the k0s binary and the metadata file
function main() {
    k0sbin
    metadata
}

main "$@"
