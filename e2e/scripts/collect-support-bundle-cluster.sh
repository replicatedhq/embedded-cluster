#!/usr/bin/env bash
set -euox pipefail

main() {
    local url="https://raw.githubusercontent.com/replicatedhq/embedded-cluster-operator/main/charts/embedded-cluster-operator/troubleshoot/cluster-support-bundle.yaml"
    if ! kubectl support-bundle --output cluster.tar.gz --interactive=false --load-cluster-specs "$url" ; then
        echo "Failed to collect cluster support bundle"
        return 1
    fi
}

export KUBECONFIG=/var/lib/k0s/pki/admin.conf
export PATH=$PATH:/var/lib/embedded-cluster/bin
main "$@"
