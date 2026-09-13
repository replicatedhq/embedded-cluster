#!/bin/bash

# Resolve an immutable EC runtime cache entry from S3, or invoke a caller
# supplied builder and publish it. manifest.json is always uploaded last and is
# the only completion marker.
set -euo pipefail

: "${RUNTIME_PLAN:?required}"
: "${OUTPUT_DIR:?required}"

CACHE_BUCKET=${EC_RUNTIME_CACHE_BUCKET:-${S3_BUCKET:-}}
: "${CACHE_BUCKET:?EC_RUNTIME_CACHE_BUCKET is required}"
CACHE_PREFIX=${CACHE_PREFIX:-ec-v2-runtime-asset/v1}
BUILD_COMMAND=${BUILD_COMMAND:-}
plan_digest=$(jq -er .planDigest "$RUNTIME_PLAN")
key_prefix="${CACHE_PREFIX}/${plan_digest}"

verify_cache() {
    ./output/bin/airgap-bundle verify-runtime --dir "$OUTPUT_DIR"
    test "$(jq -r .planDigest "$OUTPUT_DIR/manifest.json")" = "$plan_digest"
}

download_cache() {
    if ! aws s3api head-object --bucket "$CACHE_BUCKET" --key "$key_prefix/manifest.json" >/dev/null 2>&1; then
        return 1
    fi
    mkdir -p "$OUTPUT_DIR"
    aws s3 cp --no-progress "s3://${CACHE_BUCKET}/${key_prefix}/images-amd64.tar" "$OUTPUT_DIR/images-amd64.tar"
    aws s3 cp --no-progress "s3://${CACHE_BUCKET}/${key_prefix}/charts.tar.gz" "$OUTPUT_DIR/charts.tar.gz"
    aws s3 cp --no-progress "s3://${CACHE_BUCKET}/${key_prefix}/manifest.json" "$OUTPUT_DIR/manifest.json"
    verify_cache
}

put_immutable() {
    local source=$1 key=$2
    if aws s3api put-object --bucket "$CACHE_BUCKET" --key "$key" --body "$source" --if-none-match '*' >/dev/null; then
        return
    fi
    # Another workflow may have won the race. Never overwrite it; download and
    # compare before accepting the existing object.
    local existing
    existing=$(mktemp "${TMPDIR:-/tmp}/ec-runtime-object.XXXXXX")
    aws s3 cp --no-progress "s3://${CACHE_BUCKET}/${key}" "$existing"
    if ! cmp -s "$source" "$existing"; then
        echo "immutable S3 object differs: s3://${CACHE_BUCKET}/${key}" >&2
        return 1
    fi
}

make output/bin/airgap-bundle
if download_cache; then
    echo "EC runtime cache hit: s3://${CACHE_BUCKET}/${key_prefix}/"
    exit 0
fi

if [ -z "$BUILD_COMMAND" ]; then
    echo "EC runtime cache miss and BUILD_COMMAND is not set" >&2
    exit 1
fi
if [ -e "$OUTPUT_DIR" ]; then
    echo "OUTPUT_DIR must not exist on a cache miss: $OUTPUT_DIR" >&2
    exit 1
fi
mkdir -p "$OUTPUT_DIR"
export RUNTIME_PLAN OUTPUT_DIR
bash -o errexit -o nounset -o pipefail -c "$BUILD_COMMAND"
test -f "$OUTPUT_DIR/images-amd64.tar"
test -f "$OUTPUT_DIR/charts.tar.gz"

./output/bin/airgap-bundle complete-runtime \
    --plan "$RUNTIME_PLAN" \
    --images "$OUTPUT_DIR/images-amd64.tar" \
    --charts "$OUTPUT_DIR/charts.tar.gz" \
    --output "$OUTPUT_DIR/manifest.json"
verify_cache

put_immutable "$OUTPUT_DIR/images-amd64.tar" "$key_prefix/images-amd64.tar"
put_immutable "$OUTPUT_DIR/charts.tar.gz" "$key_prefix/charts.tar.gz"
put_immutable "$OUTPUT_DIR/manifest.json" "$key_prefix/manifest.json"
echo "EC runtime cache published: s3://${CACHE_BUCKET}/${key_prefix}/"
