#!/usr/bin/env bash
set -euo pipefail

main() {
    if ! kubectl support-bundle --output host.tar.gz --interactive=false /var/lib/embedded-cluster/support/host-support-bundle.yaml; then
        echo "Failed to collect local host support bundle"
        return 1
    fi

    tar -zxvf host.tar.gz
    if ! ls host/host-collectors/run-host/k0s-sysinfo.txt; then
        echo "Failed to find 'k0s sysinfo' inside the host support bundle"
        return 1
    fi

    if ! kubectl support-bundle --output cluster.tar.gz --interactive=false --load-cluster-specs; then
        echo "Failed to collect cluster support bundle"
        return 1
    fi

    tar -zxvf cluster.tar.gz
    if ! ls cluster/podlogs/embedded-cluster-operator; then
        echo "Failed to find operator logs inside the cluster support bundle"
        return 1
    fi
}

export KUBECONFIG=/var/lib/k0s/pki/admin.conf
export PATH=$PATH:/var/lib/embedded-cluster/bin
main "$@"
