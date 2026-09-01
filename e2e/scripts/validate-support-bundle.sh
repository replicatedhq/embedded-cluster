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

    bundle_tar=$(ls support-bundle-*.tar.gz)
    tar -zxvf "$bundle_tar"
    bundle_dir=${bundle_tar%.tar.gz}
    rm -rf "$bundle_tar"

    echo "checking for the k0s sysinfo file"
    if ! ls "$bundle_dir/host-collectors/run-host/k0s-sysinfo.txt"; then
        echo "Failed to find 'k0s sysinfo' inside the support bundle generated with the embedded cluster binary"
        return 1
    fi

    echo "checking for the embedded-cluster-operator logs"
    if ! ls "$bundle_dir/podlogs/embedded-cluster-operator"; then
        echo "Failed to find operator logs inside the support bundle generated with the embedded cluster binary"
        return 1
    fi

    license_file="$bundle_dir/host-collectors/embedded-cluster/license.yaml"
    echo "checking for the license file inside the support bundle"
    if ! ls "$license_file"; then
        echo "Failed to find license file inside the support bundle generated with the embedded cluster binary"
        return 1
    fi

    echo "checking that the license ID was redacted in the CLI-generated support bundle"
    if grep -q "licenseID: 2cQCFfBxG7gXDmq1yAgPSM4OViF" "$license_file"; then
        echo "License ID was not redacted in the CLI-generated support bundle"
        return 1
    fi
    if ! grep -Fq "licenseID: ***HIDDEN***" "$license_file"; then
        echo "Expected redaction marker not found in the CLI-generated support bundle"
        echo "License file contents:"
        cat "$license_file"
        return 1
    fi
}

main "$@"
