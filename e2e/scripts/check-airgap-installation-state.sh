#!/usr/bin/env bash
set -euox pipefail

DIR=/usr/local/bin
. $DIR/common.sh

main() {
    local version="$1"
    local k8s_version="$2"
    local expected_app_state="${3:-not-upgraded}"

    sleep 10 # wait for kubectl to become available

    echo "ensure that all nodes are running k8s $k8s_version"
    if ! ensure_nodes_match_kube_version "$k8s_version"; then
        echo "not all nodes are running k8s $k8s_version"
        exit 1
    fi

    echo "pods"
    kubectl get pods -A

    if ! ensure_installation_is_installed; then
        echo "installation is not installed"
        exit 1
    fi

    if ! wait_for_nginx_pods; then
        echo "Failed waiting for the application's nginx pods"
        exit 1
    fi
    if ! ensure_app_deployed_airgap "$version"; then
        exit 1
    fi
    case "$expected_app_state" in
        not-upgraded)
            if ! ensure_app_not_upgraded; then
                exit 1
            fi
            ;;
        upgraded)
            if ! ensure_app_upgraded; then
                exit 1
            fi
            # This is an application-only update. Resources present only in the
            # EC upgrade configuration must not have been installed.
            if kubectl get ns kube-state-metrics; then
                echo "found kube-state-metrics namespace after application-only upgrade"
                exit 1
            fi
            ;;
        *)
            echo "unknown expected application state: $expected_app_state"
            exit 1
            ;;
    esac

    echo "ensure that the admin console branding is available and has the DR label"
    if ! kubectl get cm -n kotsadm kotsadm-application-metadata --show-labels | grep -q 'replicated.com/disaster-recovery=infra'; then
        echo "kotsadm-application-metadata configmap not found with the DR label"
        kubectl get cm -n kotsadm --show-labels
        kubectl get cm -n kotsadm kotsadm-application-metadata -o yaml
        exit 1
    fi

    validate_all_pods_healthy
}

main "$@"
