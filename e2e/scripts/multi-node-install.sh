#!/usr/bin/env bash

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

wait_for_healthy_nodes() {
    ready=$(kubectl get nodes | grep -v NotReady | grep -c Ready)
    counter=0
    while [ "$ready" -lt "3" ]; do
        if [ "$counter" -gt 36 ]; then
            return 1
        fi
        sleep 5
        counter=$((counter+1))
        echo "Waiting for node to be ready"
        ready=$(kubectl get nodes | grep -v NotReady | grep -c Ready)
        kubectl get nodes || true
    done
    return 0
}

main() {
    echo "$config" > /root/k0sctl.yaml
    if ! helmvm install --multi-node --no-prompt --config /root/k0sctl.yaml 2>&1 | tee /tmp/log ; then
        echo "Failed to install helmvm"
        exit 1
    fi
    if ! grep -q "You can now access your cluster" /tmp/log; then
        echo "Failed to install helmvm"
        exit 1
    fi
    if ! wait_for_healthy_nodes; then
        echo "Failed to install helmvm"
        exit 1
    fi
}

export KUBECONFIG=/root/.helmvm/etc/kubeconfig
export PATH=$PATH:/root/.helmvm/bin
main
