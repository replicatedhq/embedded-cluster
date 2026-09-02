#!/bin/bash

# Build or restore the immutable EC runtime archives used by local E2E bundles.
set -euo pipefail

: "${EC_BINARY:?required}"
: "${NETWORK_MODE:?required (online or airgap)}"
: "${EC_RUNTIME_CACHE_BUCKET:?required}"
RUNTIME_ASSETS_DIR=${RUNTIME_ASSETS_DIR:-output/ec-runtime-assets}
RUNTIME_PLAN=${RUNTIME_PLAN:-output/ec-runtime-plan.json}
PLATFORM=${PLATFORM:-linux/amd64}
CHART_FILES=${CHART_FILES:-}
metadata_file=${VERSION_METADATA:-output/ec-version-metadata.json}

make output/bin/airgap-bundle
mkdir -p "$(dirname "$RUNTIME_PLAN")"
if [ -z "$CHART_FILES" ]; then
    chart_dir=$(mktemp -d "${TMPDIR:-/tmp}/ec-runtime-source-charts.XXXXXX")
    trap 'rm -rf "$chart_dir"' EXIT
    "$EC_BINARY" version metadata > "$metadata_file"
    while IFS= read -r repository; do
        [ -z "$repository" ] && continue
        name=$(jq -er .name <<<"$repository")
        url=$(jq -er .url <<<"$repository")
        repo_args=(repo add "$name" "$url" --force-update)
        username=$(jq -r '.username // empty' <<<"$repository")
        password=$(jq -r '.password // empty' <<<"$repository")
        cert_file=$(jq -r '.certFile // empty' <<<"$repository")
        key_file=$(jq -r '.keyFile // empty' <<<"$repository")
        ca_file=$(jq -r '.caFile // empty' <<<"$repository")
        insecure=$(jq -r '.insecure // false' <<<"$repository")
        [ -z "$username" ] || repo_args+=(--username "$username")
        [ -z "$password" ] || repo_args+=(--password "$password")
        [ -z "$cert_file" ] || repo_args+=(--cert-file "$cert_file")
        [ -z "$key_file" ] || repo_args+=(--key-file "$key_file")
        [ -z "$ca_file" ] || repo_args+=(--ca-file "$ca_file")
        [ "$insecure" != true ] || repo_args+=(--insecure-skip-tls-verify)
        helm "${repo_args[@]}"
    done < <(jq -c '.Configs.repositories[]?' "$metadata_file")
    while IFS=$'\t' read -r chart version; do
        [ -z "$chart" ] && continue
        helm pull "$chart" --version "$version" --destination "$chart_dir"
    done < <(jq -r '.Configs.charts[] | [.chartname,.version] | @tsv' "$metadata_file")
    CHART_FILES=$(find "$chart_dir" -type f -name '*.tgz' -print | sort)
    export CHART_FILES VERSION_METADATA="$metadata_file"
fi

# EC_BINARY must contain the release being assembled. Do not omit release
# metadata: application Config extensions and their images are runtime inputs.
mapfile -t requested_images < <("$EC_BINARY" version list-images | sed '/^[[:space:]]*$/d' | sort -u)
resolve_args=()
for image in "${requested_images[@]}"; do resolve_args+=(--image "$image"); done
./output/bin/airgap-bundle resolve-images "${resolve_args[@]}" > "$RUNTIME_PLAN.images"

plan_args=(--network-mode "$NETWORK_MODE" --platform "$PLATFORM")
while IFS= read -r item; do
    plan_args+=(--image "$(jq -r '[.reference,.repository,.digest] | join(",")' <<<"$item")")
done < <(jq -c '.[]' "$RUNTIME_PLAN.images")

chart_args=()
while IFS= read -r chart; do
    [ -z "$chart" ] && continue
    digest=$(shasum -a 256 "$chart" | awk '{print "sha256:"$1}')
    plan_args+=(--chart "$(basename "$chart"),$digest")
    chart_args+=(--chart "$chart")
done <<< "$CHART_FILES"
./output/bin/airgap-bundle runtime-plan "${plan_args[@]}" > "$RUNTIME_PLAN"

if [ "${1:-}" = _build ]; then
    ./output/bin/airgap-bundle build-runtime --plan "$RUNTIME_PLAN" --output-dir "$OUTPUT_DIR" "${chart_args[@]}"
    exit
fi

export RUNTIME_PLAN
export OUTPUT_DIR="$RUNTIME_ASSETS_DIR"
export BUILD_COMMAND="$(printf '%q ' "$0" _build)"
./scripts/resolve-ec-runtime-assets.sh
