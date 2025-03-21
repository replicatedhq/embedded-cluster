#!/usr/bin/env bash
set -euox pipefail

DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
. $DIR/common.sh

main() {
    if ! kubectl support-bundle --output host.tar.gz --interactive=false "${EMBEDDED_CLUSTER_BASE_DIR}/support/host-support-bundle.yaml" ; then
        echo "Failed to collect local host support bundle"
        return 1
    fi
}

main "$@"
