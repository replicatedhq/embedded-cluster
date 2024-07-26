#!/usr/bin/env bash
set -euox pipefail

DIR=/usr/local/bin

. $DIR/common.sh

main() {
    # We shouldn't have these files but clean up to be sure
    rm -f /var/lib/embedded-cluster/support/host-preflight-results.json
    rm -f /tmp/log

    echo "Wait for health node"
    if ! wait_for_healthy_node; then
        echo "Failed to wait for healthy node, yet we expect it to be health as a prerequisite"
        exit 1
    fi

    echo "Run installer"
    ls -al /var/lib/embedded-cluster/bin
    if ! embedded-cluster install --no-prompt 2>&1 | tee /tmp/log ; then
        cat /etc/os-release
        echo "Failed to install embedded-cluster"
        exit 1
    fi
    echo "Check if admin console is ready"
    if ! grep -q "Admin Console is ready!" /tmp/log; then
        echo "Failed to validate that the Admin Console is ready"
        exit 1
    fi
    echo "Check if host preflights results exist on disk"
    if ! has_stored_host_preflight_results; then
        echo "Install hasn't stored host preflight results to disk"
        cat /tmp/log
        exit 1
    fi
    echo "Wait for health node again"
    if ! wait_for_healthy_node; then
        echo "Failed to wait for healthy node"
        exit 1
    fi
}

export EMBEDDED_CLUSTER_METRICS_BASEURL="https://staging.replicated.app"
export KUBECONFIG=/var/lib/k0s/pki/admin.conf
export PATH=$PATH:/var/lib/embedded-cluster/bin
main
