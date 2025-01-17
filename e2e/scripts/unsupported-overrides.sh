#!/usr/bin/env bash
set -euox pipefail

DIR=/usr/local/bin
. $DIR/common.sh

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
                  localpv-provisioner:
                    analytics:
                      enabled: false
                    hostpathClass:
                      enabled: true
                      isDefaultClass: true
                  engines:
                    local:
                      lvm:
                        enabled: false
                      zfs:
                        enabled: false
                    replicated:
                      mayastor:
                        enabled: false
                version: 4.0.1
              - chartname: oci://registry.replicated.com/library/embedded-cluster-operator
                name: embedded-cluster-operator
                namespace: embedded-cluster
                order: 2
                version: 0.34.9
              - chartname: oci://registry.replicated.com/library/admin-console
                name: admin-console
                namespace: kotsadm
                order: 3
                version: 1.109.12
                values: |
                  isHA: false
                  isHelmManaged: false
                  minimalRBAC: false
                  service:
                    nodePort: 30000
                    type: NodePort
                  passwordSecretRef:
                    name: kotsadm-password
                    key: passwordBcrypt
              - chartname: oci://registry-1.docker.io/bitnamicharts/memcached
                name: memcached
                namespace: embedded-cluster
                order: 4
                version: 6.6.2
              repositories:
              - name: openebs
                url: https://openebs.github.io/openebs
"

embed_cluster_config() {
    content="$1"
    echo "$content" > /root/release.yaml
    tar -czvf /root/release.tar.gz /root/release.yaml
    embedded-cluster-release-builder /usr/local/bin/embedded-cluster /root/release.tar.gz /usr/local/bin/embedded-cluster
}

override_applied() {
    grep -A1 telemetry "$K0SCONFIG" > /tmp/telemetry-section
    if ! grep -q "enabled: true" /tmp/telemetry-section; then
      echo "override not applied, expected telemetry true, actual config:"
      cat "$K0SCONFIG"
      return 1
    fi
    if ! grep "testing-overrides-k0s-name" "$K0SCONFIG"; then
      echo "override not applied, expected name testing-overrides-k0s-name, actual config:"
      cat "$K0SCONFIG"
      return 1
    fi
    if ! grep "net.ipv4.ip_forward" "$K0SCONFIG"; then
      echo "override not applied, expected worker profile not found, actual config:"
      cat "$K0SCONFIG"
      return 1
    fi
    if ! grep "memcached" "$K0SCONFIG"; then
      echo "override not applied, expected memcached helmchart not found, actual config:"
      cat "$K0SCONFIG"
      return 1
    fi
}

main() {
    embed_cluster_config "$embedded_cluster_config"
    if ! embedded-cluster install --yes 2>&1 | tee /tmp/log ; then
        echo "Failed to install embedded-cluster"
        cat /tmp/log
        exit 1
    fi
    if ! grep -q "Admin Console is ready!" /tmp/log; then
        echo "Failed to validate that the Admin Console is ready"
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

main
