#!/usr/bin/env bash
set -euox pipefail

main() {
    tar -zxvf host.tar.gz
    if ! ls host/host-collectors/run-host/k0s-sysinfo.txt; then
        echo "Failed to find 'k0s sysinfo' inside the host support bundle"
        return 1
    fi

    tar -zxvf cluster.tar.gz
    if ! ls cluster/podlogs/embedded-cluster-operator; then
        echo "Failed to find operator logs inside the cluster support bundle"
        return 1
    fi
}

main "$@"
