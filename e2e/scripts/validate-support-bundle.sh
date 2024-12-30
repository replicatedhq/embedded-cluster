#!/usr/bin/env bash
set -euox pipefail

main() {
    tar -zxvf host.tar.gz
    if ! ls host/host-collectors/run-host/k0s-sysinfo.txt; then
        echo "Failed to find 'k0s sysinfo' inside the host support bundle"
        return 1
    fi
    rm -rf host.tar.gz

    tar -zxvf cluster.tar.gz
    if ! ls cluster/podlogs/embedded-cluster-operator; then
        echo "Failed to find operator logs inside the cluster support bundle"
        return 1
    fi
    rm -rf cluster.tar.gz

    tar -zxvf support-bundle-*.tar.gz
    rm -rf support-bundle-*.tar.gz

    echo "checking for the k0s sysinfo file"
    if ! ls support-bundle-*/host-collectors/run-host/k0s-sysinfo.txt; then
        echo "Failed to find 'k0s sysinfo' inside the support bundle generated with the embedded cluster binary"
        return 1
    fi

    echo "checking for the embedded-cluster-operator logs"
    if ! ls support-bundle-*/podlogs/embedded-cluster-operator; then
        echo "Failed to find operator logs inside the support bundle generated with the embedded cluster binary"
        return 1
    fi

    echo "checking for the license file inside the support bundle"
    if ! ls support-bundle-*/host-collectors/embedded-cluster/license.yaml; then
        echo "Failed to find license file inside the support bundle generated with the embedded cluster binary"
        return 1
    fi
}

main "$@"
