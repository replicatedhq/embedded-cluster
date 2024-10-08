#!/usr/bin/env bash
set -euox pipefail

DIR=/usr/local/bin
. $DIR/common.sh

main() {
    local pod_cidr_matcher="$1"
    local service_cidr_matcher="$2"
    sleep 10 # wait for kubectl to become available

    echo "nodes:"
    kubectl get nodes -o wide
    echo "pods cidr: $pod_cidr_matcher"
    kubectl get pods -A -o wide
    if ! kubectl get pods -A -o jsonpath='{.items[*].status.podIP}' | grep -e "$pod_cidr_matcher" ; then
        echo "pods not found in CIDR"
        return 1
    fi
    echo "services cidr: $service_cidr_matcher"
    kubectl get services -A
    if ! kubectl get services -A -o jsonpath='{.items[*].spec.clusterIP}' | grep -e "$service_cidr_matcher" ; then
        echo "services not found in CIDR"
        return 1
    fi
}

main "$@"
