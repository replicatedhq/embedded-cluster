#!/usr/bin/env bash
set -euo pipefail

preflight_with_failure="
apiVersion: troubleshoot.sh/v1beta2
kind: HostPreflight
spec:
  collectors:
    - tcpPortStatus:
        collectorName: Port 24
        port: 24
    - tcpPortStatus:
        collectorName: Port 22
        port: 22
  analyzers:
    - tcpPortStatus:
        checkName: Port 24
        collectorName: Port 24
        outcomes:
          - fail:
              when: connection-refused
              message: Connection to port 24 was refused.
          - warn:
              when: address-in-use
              message: Another process was already listening on port 24.
          - fail:
              when: connection-timeout
              message: Timed out connecting to port 24.
          - fail:
              when: error
              message: Unexpected port status
          - pass:
              when: connected
              message: Port 24 is available
          - warn:
              message: Unexpected port status
    - tcpPortStatus:
        checkName: Port 22
        collectorName: Port 22
        outcomes:
          - fail:
              when: connection-refused
              message: Connection to port 22 was refused.
          - fail:
              when: address-in-use
              message: Another process was already listening on port 22.
          - fail:
              when: connection-timeout
              message: Timed out connecting to port 22.
          - fail:
              when: error
              message: Unexpected port status
          - pass:
              when: connected
              message: Port 22 is available
          - warn:
              message: Unexpected port status
"

preflight_with_warning="
apiVersion: troubleshoot.sh/v1beta2
kind: HostPreflight
spec:
  collectors:
    - tcpPortStatus:
        collectorName: Port 24
        port: 24
    - tcpPortStatus:
        collectorName: Port 22
        port: 22
  analyzers:
    - tcpPortStatus:
        checkName: Port 24
        collectorName: Port 24
        outcomes:
          - fail:
              when: connection-refused
              message: Connection to port 24 was refused.
          - warn:
              when: address-in-use
              message: Another process was already listening on port 24.
          - fail:
              when: connection-timeout
              message: Timed out connecting to port 24.
          - fail:
              when: error
              message: Unexpected port status
          - pass:
              when: connected
              message: Port 24 is available
          - warn:
              message: Unexpected port status
    - tcpPortStatus:
        checkName: Port 22
        collectorName: Port 22
        outcomes:
          - fail:
              when: connection-refused
              message: Connection to port 22 was refused.
          - warn:
              when: address-in-use
              message: Another process was already listening on port 22.
          - fail:
              when: connection-timeout
              message: Timed out connecting to port 22.
          - fail:
              when: error
              message: Unexpected port status
          - pass:
              when: connected
              message: Port 22 is available
          - warn:
              message: Unexpected port status
"

embed_preflight() {
    content="$1"
    rm -rf /root/preflight*
    echo "$content" > /root/preflight.yaml
    tar -czvf /root/preflight.tar.gz /root/preflight.yaml
    rm -rf /usr/local/bin/embedded-cluster
    cp -Rfp /usr/local/bin/embedded-cluster-copy /usr/local/bin/embedded-cluster
    embedded-cluster-release-builder /usr/local/bin/embedded-cluster /root/preflight.tar.gz /usr/local/bin/embedded-cluster
}

has_applied_host_preflight() {
    if ! grep -q "Port 24 is available" /tmp/log ; then
        return 1
    fi
    if ! grep -q "Another process was already listening on port 22" /tmp/log ; then
        return 1
    fi
}

wait_for_healthy_node() {
    ready=$(kubectl get nodes | grep -v NotReady | grep -c Ready || true)
    counter=0
    while [ "$ready" -lt "1" ]; do
        if [ "$counter" -gt 36 ]; then
            return 1
        fi
        sleep 5
        counter=$((counter+1))
        echo "Waiting for node to be ready"
        ready=$(kubectl get nodes | grep -v NotReady | grep -c Ready || true)
        kubectl get nodes || true
    done
    return 0
}

main() {
    cp -Rfp /usr/local/bin/embedded-cluster /usr/local/bin/embedded-cluster-copy
    embed_preflight "$preflight_with_failure"
    if embedded-cluster install --no-prompt --license /tmp/license.yaml 2>&1 | tee /tmp/log ; then
        cat /tmp/log
        echo "Expected installation to fail"
        exit 1
    fi
    if ! has_applied_host_preflight; then
        echo "Install hasn't applied host preflight"
        cat /tmp/log
        exit 1
    fi
    mv /tmp/log /tmp/log-failure
    embed_preflight "$preflight_with_warning"
    if ! embedded-cluster install --no-prompt --license /tmp/license.yaml 2>&1 | tee /tmp/log ; then
        cat /etc/os-release
        echo "Failed to install embedded-cluster"
        exit 1
    fi
    if ! grep -q "Admin Console is ready!" /tmp/log; then
        echo "Failed to install embedded-cluster"
        exit 1
    fi
    if ! has_applied_host_preflight; then
        echo "Install hasn't applied host preflight"
        cat /tmp/log
        exit 1
    fi
    if ! wait_for_healthy_node; then
        echo "Failed to install embedded-cluster"
        exit 1
    fi
    if ! systemctl restart embedded-cluster; then
        echo "Failed to restart embedded-cluster service"
        exit 1
    fi
}

export EMBEDDED_CLUSTER_METRICS_BASEURL="https://staging.replicated.app"
export KUBECONFIG=/root/.config/embedded-cluster/etc/kubeconfig
export PATH=$PATH:/root/.config/embedded-cluster/bin
main
