#!/usr/bin/env bash
set -euo pipefail

DIR=/usr/local/bin
. $DIR/common.sh

check_openebs_storage_class() {
    scs=$(kubectl get sc --no-headers | wc -l)
    if [ "$scs" -ne "1" ]; then
        echo "Expected 1 storage class, found $scs"
        kubectl get sc
        return 1
    fi
}

check_airgap_pvc() {
    if ! kubectl get pvc -n registry --no-headers=true | wc -l | grep -q 1 ; then
        echo "Failed to find registry pvc"
        kubectl get pvc -A
        return 1
    fi
}

main() {
    local version="$1"
    local additional_args=
    if [ -n "${2:-}" ]; then
        additional_args="${*:2}"
        echo "Running install with additional args: $additional_args"
    fi

    echo "Running install without a license"
    if embedded-cluster install --no-prompt --airgap-bundle /assets/release.airgap $additional_args 2>&1 | tee /tmp/log ; then
        echo "Expected installation to fail without a license provided"
        exit 1
    fi

    echo "Running install with a license"
    if ! embedded-cluster install --no-prompt --license /assets/license.yaml --airgap-bundle /assets/release.airgap $additional_args 2>&1 | tee /tmp/log ; then
        echo "Failed to install embedded-cluster"
        kubectl get pods -A
        kubectl get storageclass -A
        exit 1
    fi
    if ! grep -q "Admin Console is ready" /tmp/log; then
        echo "Failed to validate that the Admin Console is ready"
        exit 1
    fi
    if ! ensure_version_metadata_present; then
        echo "Failed to check the presence of the version metadata configmap"
        exit 1
    fi
    if ! ensure_binary_copy; then
        echo "Failed to ensure the embedded binary has been copied to ${EMBEDDED_CLUSTER_BASE_DIR}/bin"
        exit 1
    fi
    if ! wait_for_healthy_node; then
        echo "Failed to wait for healthy node"
        exit 1
    fi
    if ! ensure_node_config; then
        echo "Cluster did not respect node config"
        exit 1
    fi
    if ! wait_for_pods_running 900; then
        echo "Failed to wait for pods to be running"
        exit 1
    fi
    if ! check_openebs_storage_class; then
        echo "Failed to validate if only openebs storage class is present"
        exit 1
    fi
    if ! wait_for_ingress_pods; then
        echo "Failed waiting for ingress pods"
        exit 1
    fi
    if ! ensure_app_not_upgraded; then
        exit 1
    fi
    if ! check_pod_install_order; then
        exit 1
    fi
    if ! ensure_installation_label; then
        exit 1
    fi
    if ! ensure_release_builtin_overrides; then
        exit 1
    fi
    if ! check_airgap_pvc; then
        exit 1
    fi
    if ! systemctl status embedded-cluster; then
        echo "Failed to get status of embedded-cluster service"
        exit 1
    fi

    if ! ensure_installation_is_installed; then
        echo "installation is not installed"
        exit 1
    fi

    # if this is the current version in CI
    if echo "$version" | grep -qvE "(pre-minio-removal|1.8.0-k8s|previous-stable)" ; then
        if ! ensure_license_in_data_dir; then
            exit 1
        fi
    fi

    echo "kotsadm logs"
    kubectl logs -n kotsadm -l app=kotsadm --tail=50 || true
    echo "previous kotsadm logs"
    kubectl logs -n kotsadm -l app=kotsadm --tail=50 --previous || true

    echo "all pods"
    kubectl get pods -A
}

main "$@"
