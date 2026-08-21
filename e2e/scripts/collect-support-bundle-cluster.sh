#!/usr/bin/env bash
set -euox pipefail

DIR=/usr/local/bin
. $DIR/common.sh

main() {
    export TROUBLESHOOT_AUTO_UPDATE=false

    echo "===== Upgrade/KOTS failure diagnostics ====="
    date -u +%Y-%m-%dT%H:%M:%SZ
    uname -a
    kubectl version || true
    kubectl get nodes -o wide || true
    kubectl get pods -A -o wide || true
    kubectl get events -A --sort-by=.lastTimestamp | tail -200 || true
    kubectl describe installations -A || true
    kubectl describe clusterconfigs -A || true
    kubectl logs -n kotsadm -l app=kotsadm --all-containers=true --timestamps=true --tail=-1 || true
    kubectl logs -n kotsadm -l app=kotsadm --all-containers=true --timestamps=true --previous --tail=-1 || true
    kubectl logs -n kotsadm -l app.kubernetes.io/name=embedded-cluster-upgrade --all-containers=true --timestamps=true --tail=-1 || true
    kubectl logs -n embedded-cluster -l app.kubernetes.io/name=embedded-cluster-operator --all-containers=true --timestamps=true --tail=-1 || true

    local kots_pod
    kots_pod=$(kubectl get pods -n kotsadm -l app=kotsadm -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
    if [ -n "${kots_pod}" ]; then
        echo "kotsadm pod=${kots_pod}"
        kubectl exec -n kotsadm "${kots_pod}" -- uname -a || true
        kubectl exec -n kotsadm "${kots_pod}" -- ls -la /tmp || true
        local kots_temp_files
        kots_temp_files=$(kubectl exec -n kotsadm "${kots_pod}" -- find /tmp -maxdepth 1 -type f -name 'kotsbin*' -print 2>/dev/null || true)
        for kots_temp_file in ${kots_temp_files}; do
            echo "downloaded KOTS temporary binary=${kots_temp_file}"
            kubectl exec -n kotsadm "${kots_pod}" -- ls -l "${kots_temp_file}" || true
            kubectl exec -n kotsadm "${kots_pod}" -- sha256sum "${kots_temp_file}" || true
            kubectl exec -n kotsadm "${kots_pod}" -- od -An -tx1 -N16 "${kots_temp_file}" || true
            kubectl exec -n kotsadm "${kots_pod}" -- "${kots_temp_file}" version || true
        done
    fi
    echo "===== End Upgrade/KOTS failure diagnostics ====="

    if ! kubectl support-bundle --output cluster.tar.gz --interactive=false --load-cluster-specs ; then
        if ! kubectl support-bundle --output cluster.tar.gz --interactive=false --load-cluster-specs "/automation/troubleshoot/cluster-support-bundle.yaml" ; then
            echo "Failed to collect cluster support bundle"
            return 1
        fi
    fi
}

main "$@"
