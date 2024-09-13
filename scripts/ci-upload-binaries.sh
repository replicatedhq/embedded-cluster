#!/bin/bash

set -euo pipefail

# shellcheck source=./common.sh
source ./scripts/common.sh

EC_VERSION=${EC_VERSION:-}
K0S_VERSION=${K0S_VERSION:-}
AWS_REGION="${AWS_REGION:-us-east-1}"
S3_BUCKET="${S3_BUCKET:-dev-embedded-cluster-bin}"
UPLOAD_BINARIES=${UPLOAD_BINARIES:-1}
MANGLE_METADATA=${MANGLE_METADATA:-0}

require AWS_ACCESS_KEY_ID "${AWS_ACCESS_KEY_ID}"
require AWS_SECRET_ACCESS_KEY "${AWS_SECRET_ACCESS_KEY}"
require AWS_REGION "${AWS_REGION}"
require S3_BUCKET "${S3_BUCKET}"

function init_vars() {
    if [ -z "${EC_VERSION:-}" ]; then
        EC_VERSION=$(git describe --tags --match='[0-9]*.[0-9]*.[0-9]*')
    fi
    if [ -z "${K0S_VERSION:-}" ]; then
        K0S_VERSION=$(make print-K0S_VERSION)
    fi

    require EC_VERSION "${EC_VERSION:-}"
    require K0S_VERSION "${K0S_VERSION:-}"
}

function k0sbin() {
    local k0s_override=
    k0s_override=$(make print-K0S_BINARY_SOURCE_OVERRIDE)

    # check if the binary already exists in the bucket
    local k0s_binary_exists=
    k0s_binary_exists=$(aws s3api head-object --bucket "${S3_BUCKET}" --key "k0s-binaries/${K0S_VERSION}-${ARCH}" || true)

    # if the binary already exists, we don't need to upload it again
    if [ -n "${k0s_binary_exists}" ]; then
        echo "k0s binary ${K0S_VERSION} already exists in bucket ${S3_BUCKET}, skipping upload"
        return 0
    fi

    # if the override is set, we should download this binary and upload it to the bucket so as not to require end users hit the override url
    if [ -n "${k0s_override}" ] && [ "${k0s_override}" != '' ]; then
        echo "K0S_BINARY_SOURCE_OVERRIDE is set to '${k0s_override}', using that source"
        curl --retry 5 --retry-all-errors -fL -o "build/${K0S_VERSION}" "${k0s_override}"
    else
        # download the k0s binary from official sources
        echo "downloading k0s binary from https://github.com/k0sproject/k0s/releases/download/${K0S_VERSION}/k0s-${K0S_VERSION}-${ARCH}"
        curl --retry 5 --retry-all-errors -fL -o "build/${K0S_VERSION}" "https://github.com/k0sproject/k0s/releases/download/${K0S_VERSION}/k0s-${K0S_VERSION}-${ARCH}"
    fi

    # upload the binary to the bucket
    retry 3 aws s3 cp --no-progress "build/${K0S_VERSION}" "s3://${S3_BUCKET}/k0s-binaries/${K0S_VERSION}-${ARCH}"
}

function operatorbin() {
    local operator_image=""
    local operator_version=""

    if [ ! -f "operator/build/image-$EC_VERSION" ]; then
        fail "file operator/build/image-$EC_VERSION not found"
    fi

    operator_image=$(cat "operator/build/image-$EC_VERSION")
    operator_version="${EC_VERSION#v}" # remove the 'v' prefix

    docker run --platform "linux/$ARCH" -d --name operator "$operator_image"
    mkdir -p operator/bin
    docker cp operator:/manager operator/bin/operator
    docker rm -f operator

    # compress the operator binary
    tar -czvf "build/${operator_version}.tar.gz" -C operator/bin operator

    # upload the binary to the bucket
    retry 3 aws s3 cp --no-progress "build/${operator_version}.tar.gz" "s3://${S3_BUCKET}/operator-binaries/${operator_version}-${ARCH}.tar.gz"
}

function kotsbin() {
    # first, figure out what version of kots is in the current build
    local kots_version=
    kots_version=$(make print-KOTS_VERSION)

    local kots_override=
    kots_override=$(make print-KOTS_BINARY_URL_OVERRIDE)

    # check if the binary already exists in the bucket
    local kots_binary_exists=
    kots_binary_exists=$(aws s3api head-object --bucket "${S3_BUCKET}" --key "kots-binaries/${kots_version}-${ARCH}.tar.gz" || true)

    # if the binary already exists, we don't need to upload it again
    if [ -n "${kots_binary_exists}" ]; then
        echo "kots binary ${kots_version} already exists in bucket ${S3_BUCKET}, skipping upload"
        return 0
    fi

    if [ -n "${kots_override}" ] && [ "${kots_override}" != '' ]; then
        echo "KOTS_BINARY_URL_OVERRIDE is set to '${kots_override}', using that source"
        curl --retry 5 --retry-all-errors -fL -o "build/kots_linux_${ARCH}.tar.gz" "${kots_override}"
    else
        # download the kots binary from github
        echo "downloading kots binary from https://github.com/replicatedhq/kots/releases/download/${kots_version}/kots_linux_${ARCH}.tar.gz"
        curl --retry 5 --retry-all-errors -fL -o "build/kots_linux_${ARCH}.tar.gz" "https://github.com/replicatedhq/kots/releases/download/${kots_version}/kots_linux_${ARCH}.tar.gz"
    fi

    # upload the binary to the bucket
    retry 3 aws s3 cp --no-progress "build/kots_linux_${ARCH}.tar.gz" "s3://${S3_BUCKET}/kots-binaries/${kots_version}-${ARCH}.tar.gz"
}

function metadata() {
    if [ -z "${EC_VERSION}" ]; then
        echo "EC_VERSION unset, not uploading metadata.json"
        return 0
    fi

    if [ "$MANGLE_METADATA" == "1" ]; then
        jq '(.Configs.charts[] | select(.name == "embedded-cluster-operator")).values += "resources:\n  requests:\n    cpu: 123m"' build/metadata.json > build/metadata.tmp.json
        mv build/metadata.tmp.json build/metadata.json
    fi

    # check if a file 'build/metadata.json' exists in the directory
    # if it does, upload it as metadata/v${EC_VERSION}.json
    if [ -f "build/metadata.json" ]; then
        # append a 'v' prefix to the version if it doesn't already have one
        retry 3 aws s3 cp --no-progress build/metadata.json "s3://${S3_BUCKET}/metadata/v${EC_VERSION#v}.json"
    else
        echo "build/metadata.json not found, skipping upload"
    fi
}

function embeddedcluster() {
    if [ -z "${EC_VERSION}" ]; then
        echo "EC_VERSION unset, not uploading embedded cluster release"
        return 0
    fi

    # check if a file 'build/embedded-cluster-linux-$ARCH.tgz' exists in the directory
    # if it does, upload it as releases/v${EC_VERSION}.tgz
    if [ -f "build/embedded-cluster-linux-$ARCH.tgz" ]; then
        # append a 'v' prefix to the version if it doesn't already have one
        retry 3 aws s3 cp --no-progress "build/embedded-cluster-linux-$ARCH.tgz" "s3://${S3_BUCKET}/releases/v${EC_VERSION#v}.tgz"
    else
        echo "build/embedded-cluster-linux-$ARCH.tgz not found, skipping upload"
    fi
}

# there are three files to be uploaded for each release - the k0s binary, the metadata file, and the embedded-cluster release
# the embedded cluster release does not exist for CI builds
function main() {
    init_vars
    metadata
    if [ "${UPLOAD_BINARIES}" == "1" ]; then
        mkdir -p build
        k0sbin
        operatorbin
        kotsbin
        embeddedcluster
    fi
}

main "$@"
