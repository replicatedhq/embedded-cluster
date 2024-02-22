#!/usr/bin/env bash
set -euo pipefail

main() {
    if ! kubectl support-bundle --output sb.tgz --interactive=false /var/lib/embedded-cluster/support/host-support-bundle.yaml; then
        echo "Failed to collect support bundle"
        return 1
    fi

    if [ ! -f sb.tgz ]; then
        echo "Support bundle tgz file not found"
        return 1
    fi
}

export KUBECONFIG=/var/lib/k0s/pki/admin.conf
export PATH=$PATH:/var/lib/embedded-cluster/bin
main "$@"
