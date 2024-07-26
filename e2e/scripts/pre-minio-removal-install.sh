#!/usr/bin/env bash
set -euox pipefail

DIR=/usr/local/bin
. $DIR/common.sh

maybe_install_curl() {
    if ! command -v curl; then
        apt-get update
        apt-get install -y curl
    fi
}

install_kots_cli() {
    maybe_install_curl

    # install kots CLI
    echo "installing kots cli"
    local ec_version=
    ec_version=$(embedded-cluster version | grep AdminConsole | awk '{print substr($4,2)}' | cut -d'-' -f1)
    curl "https://kots.io/install/$ec_version" | bash

}

deploy_app() {
    echo "getting apps"
    # run a no-op kots command to populate the authstring secret
    kubectl kots get apps -n kotsadm
    echo "kotsadm version"
    kubectl kots version -n kotsadm
    echo "app versions"
    kubectl kots get versions -n kotsadm embedded-cluster-smoke-test-staging-app

    echo "exporting authstring"
    # export the authstring secret
    local kotsadm_auth_string=
    kotsadm_auth_string=$(kubectl get secret -n kotsadm kotsadm-authstring -o jsonpath='{.data.kotsadm-authstring}' | base64 -d)
    echo "kotsadm_auth_string: $kotsadm_auth_string"

    echo "getting kotsadm service IP"
    # get kotsadm service IP address
    local kotsadm_ip=
    kotsadm_ip=$(kubectl get svc -n kotsadm kotsadm -o jsonpath='{.spec.clusterIP}')
    echo "kotsadm_ip: $kotsadm_ip"

    echo "getting kotsadm service port"
    # get kotsadm service port
    local kotsadm_port=
    kotsadm_port=$(kubectl get svc -n kotsadm kotsadm -o jsonpath='{.spec.ports[?(@.name=="http")].port}')
    echo "kotsadm_port: $kotsadm_port"

    echo "bypassing cluster management page"
    # bypass cluster management page
    curl -k -X POST "http://${kotsadm_ip}:${kotsadm_port}/api/v1/embedded-cluster/management" -H "Authorization: $kotsadm_auth_string"
    echo "app versions"
    kubectl kots get versions -n kotsadm embedded-cluster-smoke-test-staging-app

    echo "providing a config for the app"
    # provide a config for the app
    kubectl kots set config embedded-cluster-smoke-test-staging-app -n kotsadm --key="hostname" --value="123" --deploy
    echo "app versions"
    kubectl kots get versions -n kotsadm embedded-cluster-smoke-test-staging-app

    echo "deploying the app"
    sleep 15

    echo "app versions"
    kubectl kots get versions -n kotsadm embedded-cluster-smoke-test-staging-app

    echo "kotsadm logs"
    kubectl logs -n kotsadm -l app=kotsadm --tail=50
}

check_openebs_storage_class() {
    scs=$(kubectl get sc --no-headers | wc -l)
    if [ "$scs" -ne "1" ]; then
        echo "Expected 1 storage class, found $scs"
        kubectl get sc
        return 1
    fi
}

main() {
    local app_deploy_method="$1"

    if embedded-cluster install --no-prompt 2>&1 | tee /tmp/log ; then
        echo "Expected installation to fail without a license provided"
        exit 1
    fi

    if ! embedded-cluster install --no-prompt --license /assets/license.yaml 2>&1 | tee /tmp/log ; then
        echo "Failed to install embedded-cluster"
        exit 1
    fi
    if ! grep -q "Admin Console is ready!" /tmp/log; then
        echo "Failed to validate that the Admin Console is ready"
        exit 1
    fi
    if ! install_kots_cli; then
        echo "Failed to install kots cli"
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
    if [[ "$app_deploy_method" == "cli" ]]; then
        if ! deploy_app; then
            echo "Failed to deploy app"
            exit 1
        fi
    fi
    if ! wait_for_pods_running 900; then
        echo "Failed to wait for pods to be running"
        exit 1
    fi
    if ! check_openebs_storage_class; then
        echo "Failed to validate if only openebs storage class is present"
        exit 1
    fi
    if [[ "$app_deploy_method" == "cli" ]]; then
        if ! wait_for_nginx_pods; then
            echo "Failed waiting for the application's nginx pods"
            exit 1
        fi
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
    if ! systemctl status embedded-cluster; then
        echo "Failed to get status of embedded-cluster service"
        exit 1
    fi
}

export EMBEDDED_CLUSTER_METRICS_BASEURL="https://staging.replicated.app"
export KUBECONFIG=/var/lib/k0s/pki/admin.conf
export PATH=$PATH:/var/lib/embedded-cluster/bin
main "$@"
