#!/usr/bin/env bash
set -euox pipefail

main() {
    echo "PermitRootLogin no" >> /etc/ssh/sshd_config
    if ! systemctl restart sshd; then
        echo "Failed to restart sshd"
        return 1
    fi
    if embedded-cluster install --no-prompt 2>&1 | tee /tmp/log ; then
        cat /tmp/log
        echo "This should never happen, able to install!"
        exit 1
    fi
    if ! grep -q "You can temporarily enable root login by changing" /tmp/log ; then
        cat /tmp/log
        echo "Failed to find expected error message"
        return 1
    fi
    sed -i '/^PermitRootLogin/c\PermitRootLogin without-password' /etc/ssh/sshd_config
    if ! systemctl restart sshd; then
        echo "Failed to restart sshd"
        return 1
    fi
    if ! embedded-cluster install --no-prompt 2>&1 | tee /tmp/log ; then
        cat /tmp/log
        echo "Unable to install with root login enabled without password"
        exit 1
    fi
}

export EMBEDDED_CLUSTER_METRICS_BASEURL="https://staging.replicated.app"
export KUBECONFIG=/root/.config/embedded-cluster/etc/kubeconfig
export PATH=$PATH:/root/.config/embedded-cluster/bin
main
