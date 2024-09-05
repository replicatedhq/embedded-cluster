#!/bin/bash

set -euo pipefail

# shellcheck source=./common.sh
source ./scripts/common.sh

require OPERATOR_CHART "${OPERATOR_CHART:-}"
require OPERATOR_IMAGE "${OPERATOR_IMAGE:-}"

function update_operator_metadata() {
    local operator_chart=
    local operator_image=

    operator_chart="$OPERATOR_CHART"
    operator_image="$OPERATOR_IMAGE"

    INPUT_OPERATOR_CHART_URL=$(echo "$operator_chart" | rev | cut -d':' -f2- | rev)
    if ! echo "$INPUT_OPERATOR_CHART_URL" | grep -q "oci://" ; then
        INPUT_OPERATOR_CHART_URL="oci://$INPUT_OPERATOR_CHART_URL"
    fi
    INPUT_OPERATOR_CHART_VERSION=$(echo "$operator_chart" | rev | cut -d':' -f1 | rev)
    INPUT_OPERATOR_IMAGE=$(echo "$operator_image" | cut -d':' -f1)

    export INPUT_OPERATOR_CHART_URL
    export INPUT_OPERATOR_CHART_VERSION
    export INPUT_OPERATOR_IMAGE
    chmod +x ./output/bin/buildtools
    ./output/bin/buildtools update addon embeddedclusteroperator
}

function main() {
    update_operator_metadata
}

main "$@"
