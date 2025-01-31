#!/usr/bin/env bash
set -euox pipefail

DIR=/usr/local/bin
. $DIR/common.sh

embedded_cluster_config="
apiVersion: embeddedcluster.replicated.com/v1beta1
kind: Config
spec:

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
}

main() {
    embed_cluster_config "$embedded_cluster_config"
    if ! embedded-cluster install --yes --license /assets/license.yaml 2>&1 | tee /tmp/log ; then
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
