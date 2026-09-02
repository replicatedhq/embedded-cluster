#!/bin/bash

set -euo pipefail

: "${APPLICATION_AIRGAP_BUNDLE:?required}"
: "${EC_BINARY:?required}"
: "${RELEASE_ARCHIVE:?required}"
: "${VERSION_METADATA:?required}"
: "${RUNTIME_ASSETS_DIR:?required}"
: "${LICENSE_FILE:?required}"
: "${APP_SLUG:?required}"
: "${APP_VERSION:?required}"

OUTPUT_BUNDLE=${OUTPUT_BUNDLE:-output/e2e-airgap/${APP_SLUG}.tgz}
workdir=$(mktemp -d "${TMPDIR:-/tmp}/ec-local-airgap.XXXXXX")
trap 'rm -rf "$workdir"' EXIT
mkdir -p "$workdir/embedded-cluster" "$(dirname "$OUTPUT_BUNDLE")"

make output/bin/airgap-bundle output/bin/embedded-cluster-release-builder
./output/bin/airgap-bundle verify-runtime --dir "$RUNTIME_ASSETS_DIR"

# Both v2 installers carry release-specific data. The outer installer is the
# downloaded executable; the copy inside the .airgap bundle is named exactly as
# it is by the production builder.
./output/bin/embedded-cluster-release-builder "$EC_BINARY" "$RELEASE_ARCHIVE" "$workdir/$APP_SLUG"
./output/bin/embedded-cluster-release-builder "$EC_BINARY" "$RELEASE_ARCHIVE" "$workdir/embedded-cluster/embedded-cluster-amd64"
chmod 0755 "$workdir/$APP_SLUG" "$workdir/embedded-cluster/embedded-cluster-amd64"
cp "$VERSION_METADATA" "$workdir/embedded-cluster/version-metadata.json"
cp "$RUNTIME_ASSETS_DIR/images-amd64.tar" "$workdir/embedded-cluster/images-amd64.tar"
cp "$RUNTIME_ASSETS_DIR/charts.tar.gz" "$workdir/embedded-cluster/charts.tar.gz"

if [ -n "${EC_ARTIFACTS_DIR:-}" ]; then cp -R "$EC_ARTIFACTS_DIR" "$workdir/embedded-cluster/artifacts"; fi
if [ -n "${EC_REGISTRY_DIR:-}" ]; then cp -R "$EC_REGISTRY_DIR" "$workdir/embedded-cluster/registry"; fi

./output/bin/airgap-bundle augment \
    --application-bundle "$APPLICATION_AIRGAP_BUNDLE" \
    --ec-dir "$workdir/embedded-cluster" \
    --version-label "$APP_VERSION" \
    --output "$workdir/$APP_SLUG.airgap"
./output/bin/airgap-bundle wrap-release \
    --app-slug "$APP_SLUG" \
    --installer "$workdir/$APP_SLUG" \
    --license "$LICENSE_FILE" \
    --airgap-bundle "$workdir/$APP_SLUG.airgap" \
    --output "$OUTPUT_BUNDLE"

cp "$workdir/$APP_SLUG.airgap.manifest.json" "$OUTPUT_BUNDLE.manifest.json"
shasum -a 256 "$OUTPUT_BUNDLE" > "$OUTPUT_BUNDLE.sha256"
