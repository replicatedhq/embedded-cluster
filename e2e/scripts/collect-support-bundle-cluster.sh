#!/usr/bin/env bash
set -euox pipefail

DIR=/usr/local/bin
. $DIR/common.sh

main() {
    if ! kubectl support-bundle --output cluster.tar.gz --interactive=false --load-cluster-specs ; then
        if ! kubectl support-bundle --output cluster.tar.gz --interactive=false --load-cluster-specs "$TROUBLESHOOT_DIR/cluster-support-bundle.yaml" ; then
            echo "Failed to collect cluster support bundle"
            return 1
        fi
    fi
}

main "$@"
