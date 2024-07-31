#!/usr/bin/env bash
set -euox pipefail

DIR=/usr/local/bin
. $DIR/common.sh

check_openebs_storage_class() {
    scs=$(kubectl get sc --no-headers | wc -l)
    if [ "$scs" -ne "1" ]; then
        echo "Expected 1 storage class, found $scs"
        kubectl get sc
        return 1
    fi
}

main() {
    if embedded-cluster install --no-prompt --skip-host-preflights --license /assets/license.yaml 2>&1 | tee /tmp/log ; then
        echo "Expected installation to fail with a license provided"
        exit 1
    fi

    if ! embedded-cluster install --no-prompt --skip-host-preflights 2>&1 | tee /tmp/log ; then
        cat /etc/os-release
        echo "Failed to install embedded-cluster"
        exit 1
    fi
    if ! grep -q "Admin Console is ready!" /tmp/log; then
        echo "Failed to validate that the Admin Console is ready"
        exit 1
    fi
    if ! wait_for_healthy_node; then
        echo "Failed to wait for healthy node"
        exit 1
    fi
    if ! wait_for_pods_running 900; then
        echo "Failed to wait for pods to be running"
        exit 1
    fi
    if ! check_openebs_storage_class; then
        echo "Failed to validate if only openebs storage class is present"
        exit 1
    fi
    if ! systemctl restart embedded-cluster; then
        echo "Failed to restart embedded-cluster service"
        exit 1
    fi
}

export EMBEDDED_CLUSTER_METRICS_BASEURL="https://staging.replicated.app"
export KUBECONFIG=/var/lib/k0s/pki/admin.conf
export PATH=\$PATH:/var/lib/embedded-cluster/bin
main
