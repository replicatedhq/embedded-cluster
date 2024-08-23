#!/usr/bin/env bash
set -euox pipefail

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

export KUBECONFIG=/var/lib/k0s/pki/admin.conf
export PATH=$PATH:/var/lib/embedded-cluster/bin
main "$@"
