#!/bin/bash

set -euo pipefail

# shellcheck source=./common.sh
source ./scripts/common.sh

EC_VERSION=${EC_VERSION:-}
K0S_VERSION=${K0S_VERSION:-}
AWS_REGION="${AWS_REGION:-us-east-1}"
S3_BUCKET="${S3_BUCKET:-dev-embedded-cluster-bin}"
UPLOAD_BINARIES=${UPLOAD_BINARIES:-1}
ARCH=${ARCH:-$(go env GOARCH)}

function sha256_file() {
    if command -v sha256sum >/dev/null 2>&1; then
        sha256sum "$1"
    else
        shasum -a 256 "$1"
    fi
}

function inspect_kots_archive() {
    local archive=$1
    local label=$2
    local inspect_dir="build/kots-inspect-${label}"

    echo "===== KOTS artifact diagnostics: ${label} ====="
    echo "archive=${archive}"
    ls -l "${archive}"
    sha256_file "${archive}"
    gzip -t "${archive}"
    echo "archive members:"
    tar -tzvf "${archive}"

    mkdir -p "${inspect_dir}"
    tar -xzf "${archive}" -C "${inspect_dir}"
    if [ ! -f "${inspect_dir}/kots" ]; then
        echo "ERROR: archive does not contain a regular file named kots"
        return 1
    fi

    echo "extracted binary:"
    ls -l "${inspect_dir}/kots"
    sha256_file "${inspect_dir}/kots"
    file "${inspect_dir}/kots" || true
    echo -n "first 16 bytes: "
    od -An -tx1 -N16 "${inspect_dir}/kots"
    if command -v readelf >/dev/null 2>&1; then
        readelf -h "${inspect_dir}/kots" || true
    fi
    echo "executing extracted binary:"
    "${inspect_dir}/kots" version
    echo "===== End KOTS artifact diagnostics: ${label} ====="
}

ensure_secret "AWS_ACCESS_KEY_ID" "ARTIFACT_UPLOAD_AWS_ACCESS_KEY_ID"
ensure_secret "AWS_SECRET_ACCESS_KEY" "ARTIFACT_UPLOAD_AWS_SECRET_ACCESS_KEY"
require AWS_REGION "${AWS_REGION}"
require S3_BUCKET "${S3_BUCKET}"

function init_vars() {
    if [ -z "${EC_VERSION:-}" ]; then
        EC_VERSION=$(git describe --tags --match='[0-9]*.[0-9]*.[0-9]*' --abbrev=4)
    fi
    if [ -z "${K0S_VERSION:-}" ]; then
        K0S_VERSION=$(make print-K0S_VERSION)
    fi

    require EC_VERSION "${EC_VERSION:-}"
    require K0S_VERSION "${K0S_VERSION:-}"

    echo "===== Binary upload context ====="
    echo "git_commit=$(git rev-parse HEAD)"
    echo "EC_VERSION=${EC_VERSION}"
    echo "K0S_VERSION=${K0S_VERSION}"
    echo "KOTS_VERSION=$(make print-KOTS_VERSION)"
    echo "ARCH=${ARCH}"
    echo "runner_uname=$(uname -a)"
    echo "S3_BUCKET=${S3_BUCKET}"
    echo "AWS_REGION=${AWS_REGION}"
    echo "UPLOAD_BINARIES=${UPLOAD_BINARIES}"
    echo "GITHUB_RUN_ID=${GITHUB_RUN_ID:-local}"
    echo "GITHUB_RUN_ATTEMPT=${GITHUB_RUN_ATTEMPT:-local}"
    echo "GITHUB_JOB=${GITHUB_JOB:-local}"
    echo "GITHUB_SHA=${GITHUB_SHA:-local}"
    echo "crane_version=$(crane version 2>/dev/null || echo unavailable)"
    echo "aws_version=$(aws --version 2>&1)"
    echo "===== End binary upload context ====="
}

function k0sbin() {
    local k0s_override=
    k0s_override=$(make print-K0S_BINARY_SOURCE_OVERRIDE K0S_VERSION="${K0S_VERSION}")

    # check if the binary already exists in the bucket
    local k0s_binary_exists=
    k0s_binary_exists=$(aws s3api head-object --bucket "${S3_BUCKET}" --key "k0s-binaries/${K0S_VERSION}-${ARCH}" || true)

    # For backwards compatibility, we upload the amd64 binary to the bucket without the arch suffix
    local k0s_noarch_binary_exists=1
    if [ "${ARCH}" == "amd64" ]; then
        k0s_noarch_binary_exists=$(aws s3api head-object --bucket "${S3_BUCKET}" --key "k0s-binaries/${K0S_VERSION}" || true)
    fi

    # if the binary already exists, we don't need to upload it again
    if [ -z "${k0s_binary_exists}" ] || [ -z "${k0s_noarch_binary_exists}" ]; then
        # if the override is set, we should download this binary and upload it to the bucket so as not to require end users hit the override url
        if [ -n "${k0s_override}" ] && [ "${k0s_override}" != '' ]; then
            echo "K0S_BINARY_SOURCE_OVERRIDE is set to '${k0s_override}', using that source"
            curl --retry 5 --retry-all-errors -fL -o "build/${K0S_VERSION}" "${k0s_override}"
        else
            # download the k0s binary from official sources
            echo "downloading k0s binary from https://github.com/k0sproject/k0s/releases/download/${K0S_VERSION}/k0s-${K0S_VERSION}-${ARCH}"
            curl --retry 5 --retry-all-errors -fL -o "build/${K0S_VERSION}" "https://github.com/k0sproject/k0s/releases/download/${K0S_VERSION}/k0s-${K0S_VERSION}-${ARCH}"
        fi
    fi

    if [ -z "${k0s_binary_exists}" ]; then
        # upload the binary to the bucket
        retry 3 aws s3 cp --no-progress "build/${K0S_VERSION}" "s3://${S3_BUCKET}/k0s-binaries/${K0S_VERSION}-${ARCH}"
    fi

    if [ -z "${k0s_noarch_binary_exists}" ]; then
        # upload the amd64 binary to the bucket without the arch suffix
        retry 3 aws s3 cp --no-progress "build/${K0S_VERSION}" "s3://${S3_BUCKET}/k0s-binaries/${K0S_VERSION}"
    fi
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

    local kots_url_override=
    kots_url_override=$(make print-KOTS_BINARY_URL_OVERRIDE)

    local kots_file_override=
    kots_file_override=$(make print-KOTS_BINARY_FILE_OVERRIDE)

    # check if the binary already exists in the bucket
    local object_key="kots-binaries/${kots_version}-${ARCH}.tar.gz"
    local kots_binary_exists=
    echo "checking s3://${S3_BUCKET}/${object_key}"
    kots_binary_exists=$(aws s3api head-object --bucket "${S3_BUCKET}" --key "${object_key}" || true)

    # if the binary already exists, we don't need to upload it again
    if [ -n "${kots_binary_exists}" ]; then
        echo "kots binary ${kots_version} already exists in bucket ${S3_BUCKET}, skipping upload"
        echo "existing S3 object metadata:"
        echo "${kots_binary_exists}"
        aws s3 cp --no-progress "s3://${S3_BUCKET}/${object_key}" "build/kots_linux_${ARCH}_from_s3.tar.gz"
        inspect_kots_archive "build/kots_linux_${ARCH}_from_s3.tar.gz" "existing-s3"
        return 0
    fi

    if [ -n "${kots_url_override}" ]; then
        echo "KOTS_BINARY_URL_OVERRIDE is set to '${kots_url_override}', using that source"
        if [[ "${kots_url_override}" == http://* ]] || [[ "${kots_url_override}" == https://* ]]; then
            curl --retry 5 --retry-all-errors -fL -o "build/kots_linux_${ARCH}.tar.gz" "${kots_url_override}"
        else
            oras pull "${kots_url_override}" --output "build"
            mv build/kots.tar.gz "build/kots_linux_${ARCH}.tar.gz"
        fi
    elif [ -n "${kots_file_override}" ]; then
        echo "KOTS_BINARY_FILE_OVERRIDE is set to '${kots_file_override}', using that source"
        tar -czvf "build/kots_linux_${ARCH}.tar.gz" -C "$(dirname "${kots_file_override}")" "$(basename "${kots_file_override}")"
    else
        echo "extracting kots binary from kotsadm image"
        echo "image manifest and resolved digest:"
        crane digest "kotsadm/kotsadm:${kots_version}" --platform "linux/${ARCH}"
        crane manifest "kotsadm/kotsadm:${kots_version}" --platform "linux/${ARCH}" | jq '{mediaType, config, layers}' || true
        # Securebuild images keep /kots as a backwards-compatible symlink. Extract
        # the symlink target because tar -O does not follow symlinks and would
        # otherwise produce an empty, non-executable artifact.
        crane export "kotsadm/kotsadm:${kots_version}" --platform "linux/${ARCH}" "build/kotsadm-rootfs-${ARCH}.tar"
        echo "KOTS paths in exported image rootfs:"
        tar -tvf "build/kotsadm-rootfs-${ARCH}.tar" | grep -E '(^|[[:space:]])(\./)?(kots|usr/local/bin/kots)([[:space:]]|$)' || true
        tar -Oxf "build/kotsadm-rootfs-${ARCH}.tar" usr/local/bin/kots > build/kots
        if [ ! -s build/kots ]; then
            echo "failed to extract kots binary from kotsadm image"
            return 1
        fi
        chmod +x build/kots
        tar -czvf "build/kots_linux_${ARCH}.tar.gz" -C build kots
    fi

    inspect_kots_archive "build/kots_linux_${ARCH}.tar.gz" "before-upload"

    # upload the binary to the bucket
    retry 3 aws s3 cp --no-progress "build/kots_linux_${ARCH}.tar.gz" "s3://${S3_BUCKET}/${object_key}"
    echo "uploaded S3 object metadata:"
    aws s3api head-object --bucket "${S3_BUCKET}" --key "${object_key}"
    aws s3 cp --no-progress "s3://${S3_BUCKET}/${object_key}" "build/kots_linux_${ARCH}_from_s3.tar.gz"
    inspect_kots_archive "build/kots_linux_${ARCH}_from_s3.tar.gz" "after-upload-s3"
    echo "comparing locally generated and freshly downloaded S3 archives:"
    cmp "build/kots_linux_${ARCH}.tar.gz" "build/kots_linux_${ARCH}_from_s3.tar.gz"
}

function metadata() {
    if [ -z "${EC_VERSION}" ]; then
        echo "EC_VERSION unset, not uploading metadata.json"
        return 0
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
