#!/usr/bin/env bash
set -euox pipefail

DIR=/usr/local/bin
. $DIR/common.sh

main() {
    if ! kubectl support-bundle --output cluster.tar.gz --interactive=false --load-cluster-specs ; then
        # NOTE: this will fail in airgap but we've already failed above
        # TODO: improve this by downloading the spec through the proxy and running with a file path
        local url="https://raw.githubusercontent.com/replicatedhq/embedded-cluster-operator/main/charts/embedded-cluster-operator/troubleshoot/cluster-support-bundle.yaml"
        if ! kubectl support-bundle --output cluster.tar.gz --interactive=false --load-cluster-specs "$url" ; then
            echo "Failed to collect cluster support bundle"
            return 1
        fi
    fi
}

main "$@"
