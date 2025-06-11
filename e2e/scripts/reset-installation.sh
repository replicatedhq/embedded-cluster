#!/usr/bin/env bash
set -euox pipefail

DIR=/usr/local/bin
. $DIR/common.sh

main() {
    local additional_flags=("$@")

    if ! embedded-cluster reset --yes "${additional_flags[@]}" | tee /tmp/log ; then
        echo "Failed to uninstall embedded-cluster"
        exit 1
    fi

    if systemctl status "${EMBEDDED_CLUSTER_BIN}"; then
        echo "Unexpectedly got status of ${EMBEDDED_CLUSTER_BIN} service"
        exit 1
    fi
}

main "$@"
