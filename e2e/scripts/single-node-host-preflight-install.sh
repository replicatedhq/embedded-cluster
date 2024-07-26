#!/usr/bin/env bash
set -euox pipefail

DIR=/usr/local/bin

. $DIR/common.sh

has_stored_host_preflight_results() {
    if [ ! -f /var/lib/embedded-cluster/support/host-preflight-results.json ]; then
        return 1
    fi
}

main() {
    # We shouldn't have these files but clean up to be sure
    rm -f /var/lib/embedded-cluster/support/host-preflight-results.json
    rm -f /tmp/log

    if ! /usr/local/bin/embedded-cluster install --no-prompt 2>&1 | tee /tmp/log ; then
        cat /etc/os-release
        echo "Failed to install embedded-cluster"
        exit 1
    fi
    if ! grep -q "Admin Console is ready!" /tmp/log; then
        echo "Failed to validate that the Admin Console is ready"
        exit 1
    fi
    if ! has_stored_host_preflight_results; then
        echo "Install hasn't stored host preflight results to disk"
        cat /tmp/log
        exit 1
    fi
    if ! wait_for_healthy_node; then
        echo "Failed to wait for healthy node"
        exit 1
    fi
}

export EMBEDDED_CLUSTER_METRICS_BASEURL="https://staging.replicated.app"
export KUBECONFIG=/var/lib/k0s/pki/admin.conf
export PATH=$PATH:/var/lib/embedded-cluster/bin
main
