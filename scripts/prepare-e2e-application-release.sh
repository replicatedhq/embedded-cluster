#!/bin/bash

# Prepare an immutable application-only release specification. Creation and
# Vendor API lookup are intentionally separate so normal E2E workflows can use
# this script without permission to publish releases.
set -euo pipefail

ROLE=${ROLE:-}
RELEASE_YAML_DIR=${RELEASE_YAML_DIR:-}
OUTPUT_DIR=${OUTPUT_DIR:-output/e2e-application-release}

if [ -z "$ROLE" ] || [ -z "$RELEASE_YAML_DIR" ]; then
    echo "ROLE and RELEASE_YAML_DIR are required" >&2
    exit 1
fi

make output/bin/airgap-bundle
if [ -e "$OUTPUT_DIR" ]; then
    echo "OUTPUT_DIR already exists: $OUTPUT_DIR" >&2
    exit 1
fi
mkdir -p "$OUTPUT_DIR"

identity_args=(--role "$ROLE" --input "$RELEASE_YAML_DIR")
for chart in nginx-app redis-app; do
    chart_dir="e2e/helm-charts/$chart"
    if [ ! -d "$chart_dir" ]; then
        echo "required Helm chart directory does not exist: $chart_dir" >&2
        exit 1
    fi
    identity_args+=(--input "$chart_dir")
done

./output/bin/airgap-bundle application-identity "${identity_args[@]}" > "$OUTPUT_DIR/identity.json"
./output/bin/airgap-bundle separate-config \
    --source "$RELEASE_YAML_DIR" \
    --application "$OUTPUT_DIR/application" \
    --config "$OUTPUT_DIR/cluster-config.yaml"

for chart in nginx-app redis-app; do
    helm package -u "e2e/helm-charts/$chart" -d "$OUTPUT_DIR/application"
    chart_archives=("$OUTPUT_DIR/application/$chart"-*.tgz)
    if [ "${#chart_archives[@]}" -ne 1 ] || [ ! -s "${chart_archives[0]}" ]; then
        echo "expected exactly one packaged archive for Helm chart $chart" >&2
        exit 1
    fi
done

echo "role=$(jq -r .role "$OUTPUT_DIR/identity.json")"
echo "digest=$(jq -r .digest "$OUTPUT_DIR/identity.json")"
echo "version=$(jq -r .version "$OUTPUT_DIR/identity.json")"
