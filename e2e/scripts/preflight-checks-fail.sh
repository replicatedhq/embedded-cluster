#!/usr/bin/env bash
set -euox pipefail

DIR=/usr/local/bin
. $DIR/common.sh

main() {
    echo "Expecting failing preflight checks"
    if embedded-cluster install run-preflights --license /assets/license.yaml "$@" 2>&1 ; then
        echo "preflight_with_failure: Expected installation to fail"
        exit 1
    fi
}

main "$@"
