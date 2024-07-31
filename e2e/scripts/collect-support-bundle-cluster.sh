#!/usr/bin/env bash
set -euox pipefail

main() {
    if ! kubectl support-bundle --output cluster.tar.gz --interactive=false --load-cluster-specs; then
        echo "Failed to collect cluster support bundle"
        return 1
    fi
}

export KUBECONFIG=/var/lib/k0s/pki/admin.conf
export PATH=\$PATH:/var/lib/embedded-cluster/bin
main "$@"
