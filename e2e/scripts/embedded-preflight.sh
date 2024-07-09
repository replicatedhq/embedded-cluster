#!/usr/bin/env bash
set -euox pipefail

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
    if ! grep -q "Another process was already listening on port 22" /tmp/log ; then
        return 1
    fi
}

has_stored_host_preflight_results() {
    if [ ! -f /var/lib/embedded-cluster/support/host-preflight-results.json ]; then
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
    # cp -Rfp /usr/local/bin/embedded-cluster /usr/local/bin/embedded-cluster-copy
    # embed_preflight "$preflight_with_failure"
    # /usr/local/bin/embedded-cluster install --no-prompt --skip-host-preflights || true
    curl -L -o k0s https://ec-k0s-binaries.s3.amazonaws.com/k0s-v1.29.5%2Bk0s.0-ec.0
    chmod +x k0s
    ./k0s sysinfo 2>&1 | tee -a sysinfo.log
    cat sysinfo.log

    # if /usr/local/bin/embedded-cluster install --no-prompt 2>&1 | tee /tmp/log ; then
    #     cat /tmp/log
    #     echo "preflight_with_failure: Expected installation to fail"
    #     exit 1
    # fi
    # if ! has_applied_host_preflight; then
    #     echo "preflight_with_failure: Install hasn't applied host preflight"
    #     cat /tmp/log
    #     exit 1
    # fi
    # if ! has_stored_host_preflight_results; then
    #     echo "preflight_with_failure: Install hasn't stored host preflight results to disk"
    #     cat /tmp/log
    #     exit 1
    # fi
    # rm /var/lib/embedded-cluster/support/host-preflight-results.json
    # mv /tmp/log /tmp/log-failure
    # embed_preflight "$preflight_with_warning"
    # if ! /usr/local/bin/embedded-cluster install --no-prompt 2>&1 | tee /tmp/log ; then
    #     cat /etc/os-release
    #     echo "preflight_with_warning: Failed to install embedded-cluster"
    #     exit 1
    # fi
    # if ! grep -q "Admin Console is ready!" /tmp/log; then
    #     echo "preflight_with_warning: Failed to validate that the Admin Console is ready"
    #     exit 1
    # fi
    # if ! has_applied_host_preflight; then
    #     echo "preflight_with_warning: Install hasn't applied host preflight"
    #     cat /tmp/log
    #     exit 1
    # fi
    # if ! has_stored_host_preflight_results; then
    #     echo "preflight_with_warning: Install hasn't stored host preflight results to disk"
    #     cat /tmp/log
    #     exit 1
    # fi
    # if ! wait_for_healthy_node; then
    #     echo "Failed to wait for healthy node"
    #     exit 1
    # fi
    # if ! systemctl restart embedded-cluster; then
    #     echo "Failed to restart embedded-cluster service"
    #     exit 1
    # fi
}

export EMBEDDED_CLUSTER_METRICS_BASEURL="https://staging.replicated.app"
export KUBECONFIG=/var/lib/k0s/pki/admin.conf
export PATH=$PATH:/var/lib/embedded-cluster/bin
main
