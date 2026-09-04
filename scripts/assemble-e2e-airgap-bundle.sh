#!/bin/bash

# Assemble one complete EC v2 E2E bundle from an immutable application-only
# baseline and the release-specific EC binary under test.
set -euo pipefail

: "${ROLE:?required}"
: "${RELEASE_YAML_DIR:?required}"
: "${EC_BINARY:?required}"
: "${APP_VERSION:?required}"
: "${STANDARD_LICENSE_ID:?required}"
: "${SNAPSHOT_LICENSE_ID:?required}"
: "${EC_RUNTIME_CACHE_BUCKET:?required}"
: "${REPLICATED_API_TOKEN:?required}"

APP_SLUG=${APP_SLUG:-embedded-cluster-smoke-test-staging-app}
APP_CHANNEL=${APP_CHANNEL:-CI-airgap}
APP_CHANNEL_ID=${APP_CHANNEL_ID:-}
APP_CHANNEL_SLUG=${APP_CHANNEL_SLUG:-ci-airgap}
APP_ID=${APP_ID:-2bViecGO8EZpChcGPeW5jbWKw2B}
MARKET_ORIGIN=${MARKET_ORIGIN:-https://staging.replicated.app}
EC_COMPONENT_BUCKET=${EC_COMPONENT_BUCKET:-tf-staging-embedded-cluster-bin}
OUTPUT_DIR=${OUTPUT_DIR:-output/e2e-airgap-bundles}

workdir=$(mktemp -d "${TMPDIR:-/tmp}/assemble-e2e-airgap.XXXXXX")
registry_id=
cleanup() {
    [ -z "$registry_id" ] || docker rm -f "$registry_id" >/dev/null 2>&1 || true
    rm -rf "$workdir"
}
trap cleanup EXIT

prepared="$workdir/prepared"
ROLE="$ROLE" RELEASE_YAML_DIR="$RELEASE_YAML_DIR" OUTPUT_DIR="$prepared" \
    ./scripts/prepare-e2e-application-release.sh
baseline_version=$(jq -er .version "$prepared/identity.json")

export REPLICATED_APP=$APP_SLUG
export REPLICATED_API_ORIGIN=${REPLICATED_API_ORIGIN:-https://api.staging.replicated.com/vendor}
if [ -z "$APP_CHANNEL_ID" ]; then
    APP_CHANNEL_ID=$(replicated channel ls --output json | \
        jq -er --arg channel "$APP_CHANNEL" '.[] | select(.name == $channel) | .id')
fi
encoded_baseline_version=$(jq -rn --arg version "$baseline_version" '$version | @uri')
release_api_path="/v3/app/$APP_ID/channel/$APP_CHANNEL_ID/releases?versionLabel=$encoded_baseline_version"
matching_releases=$(replicated api get "$release_api_path" | jq -c --arg version "$baseline_version" \
    '[.releases[]? | select(.semver == $version)] | sort_by(.channelSequence)')
release_count=$(jq -r length <<<"$matching_releases")
if [ "$release_count" -gt 1 ]; then
    echo "found $release_count releases with duplicate version label $baseline_version" >&2
    exit 1
fi
release=$(jq -c '.[0] // empty' <<<"$matching_releases")
if [ -z "$release" ]; then
    if ! replicated release create --yaml-dir "$prepared/application" --promote "$APP_CHANNEL_ID" \
        --version "$baseline_version" --wait-for-airgap; then
        # A matching concurrent creator is accepted by the exact-version lookup below.
        echo "release creation failed; checking for a concurrent exact release" >&2
    fi
fi
# Read the exact immutable release and use its channel sequence for downloads.
matching_releases=$(replicated api get "$release_api_path" | jq -c --arg version "$baseline_version" \
    '[.releases[]? | select(.semver == $version)] | sort_by(.channelSequence)')
release=$(jq -ec '
    if length == 1 then .[0]
    elif length == 0 then error("release with the requested exact version label was not found")
    else error("multiple releases have the requested exact version label")
    end
' \
    <<<"$matching_releases")
channel_sequence=$(jq -er .channelSequence <<<"$release")

download_url() {
    local license=$1 url=$2 output=$3
    local attempt http_status curl_status temporary_output
    local curl_args=(--silent --show-error --location)
    if [ -n "$license" ]; then
        curl_args+=(--header "Authorization: $license")
    fi
    temporary_output="$output.partial"
    for attempt in $(seq 1 3); do
        curl_status=0
        http_status=$(curl "${curl_args[@]}" \
            --output "$temporary_output" \
            --write-out '%{http_code}' \
            "$url") || curl_status=$?
        if [ "$curl_status" -ne 0 ]; then
            rm -f "$temporary_output"
            echo "failed to download $url: curl exited with status $curl_status" >&2
            return "$curl_status"
        fi
        if [[ "$http_status" == 2* ]]; then
            mv "$temporary_output" "$output"
            return
        fi
        rm -f "$temporary_output"
        if [ "$http_status" != 500 ]; then
            echo "failed to download $url: HTTP $http_status" >&2
            return 1
        fi
        if [ "$attempt" -eq 3 ]; then
            break
        fi
        echo "application bundle returned HTTP 500; retrying ($attempt/3)" >&2
        sleep 20
    done
    echo "application bundle still returned HTTP 500 after 3 attempts" >&2
    return 1
}

# Resolve the application-only airgap bundle by its exact channel sequence.
# This is the non-Embedded-Cluster Market API; its response is JSON containing
# a short-lived object URL.
mkdir "$workdir/baseline"
download_url "$STANDARD_LICENSE_ID" \
    "$MARKET_ORIGIN/market/v3/airgap/images/url?channel_sequence=$channel_sequence" \
    "$workdir/application-airgap-url.json"
application_airgap_url=$(jq -er .url "$workdir/application-airgap-url.json")
download_url '' "$application_airgap_url" "$workdir/baseline/$APP_SLUG.airgap"
test -s "$workdir/baseline/$APP_SLUG.airgap"

# Download customer licenses directly. Local bundle assembly must not depend on
# an Embedded Cluster release having been built by the Market API.
download_url "$STANDARD_LICENSE_ID" \
    "$MARKET_ORIGIN/customer/license/download/$APP_SLUG" \
    "$workdir/baseline/license.yaml"

mkdir "$workdir/snapshot"
download_url "$SNAPSHOT_LICENSE_ID" \
    "$MARKET_ORIGIN/customer/license/download/$APP_SLUG" \
    "$workdir/snapshot/license.yaml"

make output/bin/airgap-bundle output/bin/embedded-cluster-release-builder
mkdir "$workdir/release"
cp -R "$prepared/application/." "$workdir/release/"
cp "$prepared/cluster-config.yaml" "$workdir/release/cluster-config.yaml"
{
    echo '# channel release object'
    echo "channelID: \"$APP_CHANNEL_ID\""
    echo "channelSlug: \"$APP_CHANNEL_SLUG\""
    echo "channelSequence: $channel_sequence"
    echo "appSlug: \"$APP_SLUG\""
    echo "versionLabel: \"$APP_VERSION\""
    echo 'airgap: true'
    echo 'defaultDomains:'
    echo '  replicatedAppDomain: "staging.replicated.app"'
    echo '  proxyRegistryDomain: "proxy.staging.replicated.com"'
    echo '  replicatedRegistryDomain: "registry.staging.replicated.com"'
} > "$workdir/release/release.yaml"

"$EC_BINARY" version metadata --omit-release-metadata > "$workdir/version-metadata.json"
ec_version=$(jq -er .Versions.Installer "$workdir/version-metadata.json")
encoded_version=${ec_version#v}
encoded_version=${encoded_version//+/%2B}
release_url="https://$EC_COMPONENT_BUCKET.s3.amazonaws.com/releases/v${encoded_version}.tgz"
metadata_url="https://$EC_COMPONENT_BUCKET.s3.amazonaws.com/metadata/v${encoded_version}.json"
sed -i.bak \
    -e "s|__version_string__|$ec_version|g" \
    -e "s|__release_url__|$release_url|g" \
    -e "s|__metadata_url__|$metadata_url|g" \
    "$workdir/release/cluster-config.yaml"
find "$workdir/release" -name '*.bak' -delete
./output/bin/airgap-bundle pack-directory --dir "$workdir/release" --output "$workdir/release.tgz"
./output/bin/embedded-cluster-release-builder "$EC_BINARY" "$workdir/release.tgz" "$workdir/embedded-cluster"
chmod 0755 "$workdir/embedded-cluster"

RUNTIME_ASSETS_DIR="$workdir/runtime" RUNTIME_PLAN="$workdir/runtime-plan.json" \
    VERSION_METADATA="$workdir/version-metadata.json" NETWORK_MODE=airgap \
    EC_BINARY="$workdir/embedded-cluster" ./scripts/build-ec-runtime-assets.sh

mkdir -p "$workdir/artifacts"
download_artifact() {
    local name=$1 value url s3_key encoded_value
    value=$(jq -r ".Artifacts.$name // empty" "$workdir/version-metadata.json")
    [ -n "$value" ] || return 0
    case "$value" in
        "https://$EC_RUNTIME_CACHE_BUCKET.s3.amazonaws.com/"*)
            s3_key=${value#"https://$EC_RUNTIME_CACHE_BUCKET.s3.amazonaws.com/"}
            s3_key=${s3_key//%2B/+}
            s3_key=${s3_key//%2b/+}
            aws s3api get-object --bucket "$EC_RUNTIME_CACHE_BUCKET" --key "$s3_key" \
                "$workdir/artifacts/$name.tar.gz" >/dev/null
            return
            ;;
        http://*|https://*)
            url=$value
            ;;
        *)
            encoded_value=${value//+/%2B}
            url="https://$EC_COMPONENT_BUCKET.s3.amazonaws.com/$encoded_value"
            ;;
    esac
    curl --fail --location --retry 5 "$url" --output "$workdir/artifacts/$name.tar.gz"
}
for artifact in kots operator manager; do download_artifact "$artifact"; done

registry_dir=
lam_image=$(jq -r '.Artifacts["local-artifact-mirror-image"] // empty' "$workdir/version-metadata.json")
if [ -n "$lam_image" ]; then
    mkdir -p "$workdir/registry"
    registry_id=$(docker run -d --user "$(id -u):$(id -g)" -p 127.0.0.1:5000:5000 \
        -v "$workdir/registry:/var/lib/registry" registry:2)
    lam_dest=${lam_image#*/anonymous/}
    crane copy "$lam_image" "localhost:5000/$lam_dest" --insecure
    docker stop "$registry_id" >/dev/null
    registry_id=
    registry_dir="$workdir/registry"
fi

for variant in standard snapshot; do
    license="$workdir/baseline/license.yaml"
    [ "$variant" = snapshot ] && license="$workdir/snapshot/license.yaml"
    mkdir -p "$OUTPUT_DIR/$variant"
    APPLICATION_AIRGAP_BUNDLE="$workdir/baseline/$APP_SLUG.airgap" \
        EC_BINARY="$EC_BINARY" RELEASE_ARCHIVE="$workdir/release.tgz" \
        VERSION_METADATA="$workdir/version-metadata.json" RUNTIME_ASSETS_DIR="$workdir/runtime" \
        LICENSE_FILE="$license" APP_SLUG="$APP_SLUG" APP_VERSION="$APP_VERSION" \
        EC_ARTIFACTS_DIR="$workdir/artifacts" \
        EC_REGISTRY_DIR="$registry_dir" OUTPUT_BUNDLE="$OUTPUT_DIR/$variant/$APP_VERSION.tgz" \
        ./scripts/build-local-e2e-airgap-bundle.sh
done
