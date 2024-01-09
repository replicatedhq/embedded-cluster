#!/usr/bin/env bash
set -euo pipefail

embedded_cluster_config="
apiVersion: embeddedcluster.replicated.com/v1beta1
kind: Config
spec:
  unsupportedOverrides:
    k0s: |
      config:
        metadata:
          name: testing-overrides-k0s-name
        spec:
          telemetry:
            enabled: true
          workerProfiles:
          - name: ip-forward
            values:
              allowedUnsafeSysctls:
              - net.ipv4.ip_forward
          extensions:
            helm:
              charts:
              - chartname: openebs/openebs
                name: openebs
                namespace: openebs
                order: 1
                values: |
                  localprovisioner:
                    hostpathClass:
                      isDefaultClass: true
                  ndm:
                    enabled: false
                  ndmOperator:
                    enabled: false
                version: 3.10.0
              - chartname: oci://registry.replicated.com/library/embedded-cluster-operator
                name: embedded-cluster-operator
                namespace: embedded-cluster
                order: 2
                version: 0.13.0
              - chartname: oci://registry.replicated.com/library/admin-console
                name: admin-console
                namespace: embedded-cluster
                order: 3
                version: 1.105.1
                values: |
                  isHelmManaged: false
                  kotsApplication: default value
                  minimalRBAC: false
                  service:
                    nodePort: 30000
                    type: NodePort
              - chartname: oci://registry-1.docker.io/bitnamicharts/memcached
                name: memcached
                namespace: embedded-cluster
                order: 4
                version: 6.6.2
              repositories:
              - name: openebs
                url: https://openebs.github.io/charts
"

embed_cluster_config() {
    content="$1"
    echo "$content" > /root/release.yaml
    tar -czvf /root/release.tar.gz /root/release.yaml
    objcopy --input-target binary --output-target binary --rename-section .data=sec_bundle /root/release.tar.gz /root/release.o
    objcopy --update-section sec_bundle=/root/release.o /usr/local/bin/embedded-cluster
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

override_applied() {
    grep -A1 telemetry "$K0SCTLCONFIG" > /tmp/telemetry-section
    if ! grep -q "enabled: true" /tmp/telemetry-section; then
      echo "override not applied, expected telemetry true, actual config:"
      cat "$K0SCTLCONFIG"
      return 1
    fi
    if ! grep "testing-overrides-k0s-name" "$K0SCTLCONFIG"; then
      echo "override not applied, expected name testing-overrides-k0s-name, actual config:"
      cat "$K0SCTLCONFIG"
      return 1
    fi
    if ! grep "net.ipv4.ip_forward" "$K0SCTLCONFIG"; then
      echo "override not applied, expected worker profile not found, actual config:"
      cat "$K0SCTLCONFIG"
      return 1
    fi
    if ! grep "memcached" "$K0SCTLCONFIG"; then
      echo "override not applied, expected memcached helmchart not found, actual config:"
      cat "$K0SCTLCONFIG"
      return 1
    fi
}

wait_for_memcached_pods() {
    ready=$(kubectl get pods -n embedded-cluster | grep -c memcached || true)
    counter=0
    while [ "$ready" -lt "1" ]; do
        if [ "$counter" -gt 36 ]; then
            return 1
        fi
        sleep 5
        counter=$((counter+1))
        echo "Waiting for memcached pods"
        ready=$(kubectl get pods -n embedded-cluster | grep -c memcached || true)
        kubectl get pods -n embedded-cluster 2>&1 || true
        echo "$ready"
    done
}

main() {
    cp -Rfp /usr/local/bin/embedded-cluster /usr/local/bin/embedded-cluster-copy
    embed_cluster_config "$embedded_cluster_config"
    if ! embedded-cluster install --no-prompt 2>&1 | tee /tmp/log ; then
        echo "Failed to install embedded-cluster"
        cat /tmp/log
        exit 1
    fi
    if ! grep -q "Admin Console is ready!" /tmp/log; then
        echo "Failed to install embedded-cluster"
        exit 1
    fi
    if ! override_applied; then
        echo "Expected override to be applied"
        exit 1
    fi
    if ! wait_for_memcached_pods; then
        echo "Failed waiting for memcached pods"
        exit 1
    fi
}

export EMBEDDED_CLUSTER_METRICS_BASEURL="https://staging.replicated.app"
export KUBECONFIG=/root/.config/embedded-cluster/etc/kubeconfig
export K0SCTLCONFIG=/root/.config/embedded-cluster/etc/k0sctl.yaml
export PATH=$PATH:/root/.config/embedded-cluster/bin
main
