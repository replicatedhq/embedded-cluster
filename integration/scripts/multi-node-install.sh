#!/bin/bash

config="
apiVersion: k0sctl.k0sproject.io/v1beta1
kind: Cluster
metadata:
  name: helmvm
spec:
  hosts:
  - ssh:
      address: 10.0.0.2
      user: root
      port: 22
      keyPath: /root/.ssh/id_rsa
    role: controller+worker
    uploadBinary: true
    installFlags:
    - --disable-components konnectivity-server
    noTaints: true
  - ssh:
      address: 10.0.0.3
      user: root
      port: 22
      keyPath: /root/.ssh/id_rsa
    role: controller+worker
    uploadBinary: true
    installFlags:
    - --disable-components konnectivity-server
    noTaints: true
  - ssh:
      address: 10.0.0.4
      user: root
      port: 22
      keyPath: /root/.ssh/id_rsa
    role: controller+worker
    uploadBinary: true
    installFlags:
    - --disable-components konnectivity-server
    noTaints: true
  k0s:
    version: v1.27.2+k0s.0
    dynamicConfig: false
    config:
      apiVersion: k0s.k0sproject.io/v1beta1
      kind: ClusterConfig
      metadata:
        name: helmvm
      spec:
        network:
          provider: calico
        telemetry:
          enabled: false
"

main() {
    echo "$config" > /tmp/k0sctl.yaml
    if ! /usr/local/bin/helmvm install --multi-node --no-prompt --config /tmp/k0sctl.yaml >/tmp/log 2>&1; then
        cat /tmp/log
        echo "Failed to install helmvm"
        exit 1
    fi
    cat /tmp/log
    if ! grep -q "You can now access your cluster" /tmp/log; then
        echo "Failed to install helmvm"
        exit 1
    fi
    /usr/local/bin/wait-for-ready-nodes.sh 3
}

main
