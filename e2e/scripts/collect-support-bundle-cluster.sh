#!/usr/bin/env bash
set -euox pipefail

DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
. $DIR/common.sh

main() {
    if ! kubectl support-bundle --output cluster.tar.gz --interactive=false --load-cluster-specs ; then
        if ! kubectl support-bundle --output cluster.tar.gz --interactive=false --load-cluster-specs "/automation/troubleshoot/cluster-support-bundle.yaml" ; then
            echo "Failed to collect cluster support bundle"
            return 1
        fi
    fi
}

main "$@"
