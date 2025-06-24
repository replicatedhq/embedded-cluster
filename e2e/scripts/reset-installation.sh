#!/usr/bin/env bash
set -euox pipefail

DIR=/usr/local/bin
. $DIR/common.sh

main() {
    local additional_flags=("$@")

    if ! "${EMBEDDED_CLUSTER_BASE_DIR}/bin/${EMBEDDED_CLUSTER_BIN}" reset --yes "${additional_flags[@]}" | tee /tmp/log ; then
        echo "Failed to uninstall embedded-cluster"
        exit 1
    fi

    if systemctl status "${EMBEDDED_CLUSTER_BIN}"; then
        echo "Unexpectedly got status of embedded-cluster service"
        exit 1
    fi
}

main "$@"
