#!/usr/bin/env bash
set -euox pipefail

DIR=/usr/local/bin
. $DIR/common.sh

main() {
    local additional_args=
    if [ -n "${2:-}" ]; then
        additional_args="${*:2}"
        echo "Running install with additional args: $additional_args"
    fi

    if ! EC_DRY_RUN="true" embedded-cluster install --no-prompt --license /assets/license.yaml $additional_args 2>&1 | tee /tmp/log ; then
        echo "Failed to install embedded-cluster"
        exit 1
    fi
}

main "$@"
