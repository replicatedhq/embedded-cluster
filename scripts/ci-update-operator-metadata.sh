#!/bin/bash

set -euo pipefail

function require() {
    if [ -z "$2" ]; then
        echo "validation failed: $1 unset"
        exit 1
    else
        echo "$1 is set to $2"
    fi
}

require OPERATOR_CHART "${OPERATOR_CHART}"
require OPERATOR_IMAGE "${OPERATOR_IMAGE}"

function update_operator_metadata() {
    chmod +x ./output/bin/buildtools
    INPUT_OPERATOR_CHART_URL=$(echo "$OPERATOR_CHART" | rev | cut -d':' -f2- | rev)
    if ! echo "$INPUT_OPERATOR_CHART_URL" | grep -q "oci://" ; then
        INPUT_OPERATOR_CHART_URL="oci://$INPUT_OPERATOR_CHART_URL"
    fi
    INPUT_OPERATOR_CHART_VERSION=$(echo "$OPERATOR_CHART" | rev | cut -d':' -f1 | rev)
    INPUT_OPERATOR_IMAGE=$(echo "$OPERATOR_IMAGE" | cut -d':' -f1)
    export INPUT_OPERATOR_CHART_URL
    export INPUT_OPERATOR_CHART_VERSION
    export INPUT_OPERATOR_IMAGE
    ./output/bin/buildtools update addon embeddedclusteroperator
}

function main() {
    update_operator_metadata
}

main "$@"
