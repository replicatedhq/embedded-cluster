#!/usr/bin/env bash
set -euox pipefail

DIR=/usr/local/bin
. $DIR/common.sh

main() {
    echo "Expecting failing preflight checks"

    local additional_args=
    if [ -n "${1:-}" ]; then
        additional_args="${*:1}"
        echo "Running install with additional args: $additional_args"
    fi
    if embedded-cluster install run-preflights --license /assets/license.yaml "$additional_args" 2>&1 ; then
        echo "preflight_with_failure: Expected installation to fail"
        exit 1
    fi
}

main "$@"
