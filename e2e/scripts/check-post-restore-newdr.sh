#!/usr/bin/env bash
set -euox pipefail

DIR=/usr/local/bin
. $DIR/common.sh

main() {
    if ! kubectl -n "$APP_NAMESPACE" get pods -oyaml | grep -q restore-hook-init1 ; then
        echo "restore hook init container not found"
        exit 1
    fi
    echo "found restore hook init container"
}

main "$@"
