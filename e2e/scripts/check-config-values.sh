#!/usr/bin/env bash
set -euox pipefail

DIR=/usr/local/bin
. $DIR/common.sh

main() {
    local hostname="$1"
    local pw="$2"

    sleep 10 # wait for kubectl to become available

    echo "ensure that the nginx deployment has the correct hostname"
    kubectl get deployment nginx -n kotsadm -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="CONFIG_HOSTNAME")].value}'
    if ! kubectl get deployment nginx -n kotsadm -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="CONFIG_HOSTNAME")].value}' | grep -q "$hostname" ; then
        echo "nginx deployment does not have the correct hostname"
        return 1
    fi

    echo "ensure that the nginx deployment has the correct password"
    kubectl get deployment nginx -n kotsadm -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="CONFIG_PASSWORD")].value}'
    if ! kubectl get deployment nginx -n kotsadm -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="CONFIG_PASSWORD")].value}' | grep -q "$pw" ; then
        echo "nginx deployment does not have the correct password"
        return 1
    fi
}

main "$@"
