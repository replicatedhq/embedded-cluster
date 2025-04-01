#!/bin/bash

# This script is meant to be run after deleting the first control plane node to ensure the cluster
# is still functional.

set -euxo pipefail

DIR=/usr/local/bin
. $DIR/common.sh

function main() {
    local worker_node=
    worker_node=$(kubectl get nodes -l node-role.kubernetes.io/control-plane!=true -oname | awk -F/ 'NR==1{print $2}')

    if [ -z "$worker_node" ]; then
        echo "No worker node found"
        exit 1
    fi

    local kotsadm_image=
    kotsadm_image=$(kubectl -n kotsadm get deploy kotsadm  -o jsonpath='{.spec.template.spec.containers[0].image}')

    if [ -z "$kotsadm_image" ]; then
        echo "No kotsadm image found"
        exit 1
    fi

    # run the pod on a worker node
    kubectl run test-nllb --image "$kotsadm_image" \
        --overrides='{"spec": { "nodeSelector": {"kubernetes.io/hostname": "'"$worker_node"'"}}}' --command -- sleep infinity

    # wait for the pod to be running
    if ! kubectl wait --for=condition=ready pod/test-nllb --timeout=1m; then
        echo "Pod test-nllb did not become ready"
        kubectl describe pod test-nllb
        exit 1
    fi
}

main "$@"
