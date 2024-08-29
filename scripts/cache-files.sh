#!/bin/bash

set -euox pipefail

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
    k0s_override=$(awk '/^K0S_BINARY_SOURCE_OVERRIDE/{gsub("\"", "", $3); print $3}' Makefile)

    # check if the binary already exists in the bucket
    local k0s_binary_exists=
    k0s_binary_exists=$(aws s3api head-object --bucket "${S3_BUCKET}" --key "k0s-binaries/${k0s_version}" || true)

    # if the binary already exists, we don't need to upload it again
    if [ -n "${k0s_binary_exists}" ]; then
        echo "k0s binary ${k0s_version} already exists in bucket ${S3_BUCKET}, skipping upload"
        return 0
    fi

    # if the override is set, we should download this binary and upload it to the bucket so as not to require end users hit the override url
    if [ -n "${k0s_override}" ] && [ "${k0s_override}" != '' ]; then
        echo "K0S_BINARY_SOURCE_OVERRIDE is set to '${k0s_override}', using that source"
        curl --fail-with-body --retry 5 --retry-all-errors -fL -o "${k0s_version}" "${k0s_override}"
    else
        # download the k0s binary from official sources
        echo "downloading k0s binary from https://github.com/k0sproject/k0s/releases/download/${k0s_version}/k0s-${k0s_version}-amd64"
        curl --fail-with-body --retry 5 --retry-all-errors -fL -o "${k0s_version}" "https://github.com/k0sproject/k0s/releases/download/${k0s_version}/k0s-${k0s_version}-amd64"
    fi

    # upload the binary to the bucket
    retry 3 aws s3 cp --no-progress "${k0s_version}" "s3://${S3_BUCKET}/k0s-binaries/${k0s_version}"
}

function operatorbin() {
    docker run --platform linux/amd64 -d --name operator "$OPERATOR_IMAGE"
    mkdir -p operator/bin
    docker cp operator:/manager operator/bin/operator
    docker rm -f operator

    # compress the operator binary
    tar -czvf "${OPERATOR_VERSION}.tar.gz" -C operator/bin operator

    # upload the binary to the bucket
    retry 3 aws s3 cp --no-progress "${OPERATOR_VERSION}.tar.gz" "s3://${S3_BUCKET}/operator-binaries/${OPERATOR_VERSION}.tar.gz"
}

function kotsbin() {
    # first, figure out what version of kots is in the current build
    local kots_version=
    kots_version=$(make print-KOTS_VERSION)

    local kots_override=
    kots_override=$(awk '/^KOTS_BINARY_URL_OVERRIDE/{gsub("\"", "", $3); print $3}' Makefile)

    # check if the binary already exists in the bucket
    local kots_binary_exists=
    kots_binary_exists=$(aws s3api head-object --bucket "${S3_BUCKET}" --key "kots-binaries/${kots_version}.tar.gz" || true)

    # if the binary already exists, we don't need to upload it again
    if [ -n "${kots_binary_exists}" ]; then
        echo "kots binary ${kots_version} already exists in bucket ${S3_BUCKET}, skipping upload"
        return 0
    fi

    if [ -n "${kots_override}" ] && [ "${kots_override}" != '' ]; then
        echo "KOTS_BINARY_URL_OVERRIDE is set to '${kots_override}', using that source"
        curl --fail-with-body --retry 5 --retry-all-errors -fL -o "kots_linux_amd64.tar.gz" "${kots_override}"
    else
        # download the kots binary from github
        echo "downloading kots binary from https://github.com/replicatedhq/kots/releases/download/${kots_version}/kots_linux_amd64.tar.gz"
        curl --fail-with-body --retry 5 --retry-all-errors -fL -o "kots_linux_amd64.tar.gz" "https://github.com/replicatedhq/kots/releases/download/${kots_version}/kots_linux_amd64.tar.gz"
    fi

    # upload the binary to the bucket
    retry 3 aws s3 cp --no-progress "kots_linux_amd64.tar.gz" "s3://${S3_BUCKET}/kots-binaries/${kots_version}.tar.gz"
}

function metadata() {
    if [ -z "${EC_VERSION}" ]; then
        echo "EC_VERSION unset, not uploading metadata.json"
        return 0
    fi

    # append a 'v' prefix to the version if it doesn't already have one
    local version="$EC_VERSION"
    if ! echo "$version" | grep -q "^v"; then
        version="v$version"
    fi

    # check if a file 'metadata.json' exists in the directory
    # if it does, upload it as metadata/${version}.json
    if [ -f metadata.json ]; then
        retry 3 aws s3 cp --no-progress metadata.json "s3://${S3_BUCKET}/metadata/${version}.json"
    else
        echo "metadata.json not found, skipping upload"
    fi

}

function embeddedcluster() {
    if [ -z "${EC_VERSION}" ]; then
        echo "EC_VERSION unset, not uploading embedded cluster release"
        return 0
    fi

    # check if a file 'embedded-cluster-linux-amd64.tgz' exists in the directory
    # if it does, upload it as releases/${version}.tgz
    if [ -f embedded-cluster-linux-amd64.tgz ]; then
        retry 3 aws s3 cp --no-progress embedded-cluster-linux-amd64.tgz "s3://${S3_BUCKET}/releases/${version}.tgz"
    else
        echo "embedded-cluster-linux-amd64.tgz not found, skipping upload"
    fi
}

# there are three files to be uploaded for each release - the k0s binary, the metadata file, and the embedded-cluster release
# the embedded cluster release does not exist for CI builds
function main() {
    k0sbin
    operatorbin
    kotsbin
    metadata
    embeddedcluster
}

main "$@"
