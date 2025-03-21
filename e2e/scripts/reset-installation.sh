#!/usr/bin/env bash
set -euox pipefail

DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
. $DIR/common.sh

main() {
    local additional_flags=("$@")

    if ! "${EMBEDDED_CLUSTER_BIN}" reset --yes "${additional_flags[@]}" | tee /tmp/log ; then
        echo "Failed to uninstall embedded-cluster"
        exit 1
    fi

    if systemctl status embedded-cluster; then
        echo "Unexpectedly got status of embedded-cluster service"
        exit 1
    fi
}

main "$@"
