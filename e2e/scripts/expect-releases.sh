#!/usr/bin/env bash
set -euox pipefail

DIR=/usr/local/bin
. $DIR/common.sh

main() {
    local expected_count="$1"

    if [ -z "$expected_count" ]; then
        echo "usage: expect-releases.sh <expected_count>"
        exit 1
    fi

    echo "getting kotsadm auth string"
    local kotsadm_auth_string=
    kotsadm_auth_string=$(kubectl get secret -n kotsadm kotsadm-authstring -o jsonpath='{.data.kotsadm-authstring}' | base64 -d)
    echo "kotsadm_auth_string: $kotsadm_auth_string"

    echo "getting kotsadm service IP"
    local kotsadm_ip=
    kotsadm_ip=$(kubectl get svc -n kotsadm kotsadm -o jsonpath='{.spec.clusterIP}')
    echo "kotsadm_ip: $kotsadm_ip"

    echo "getting kotsadm service port"
    local kotsadm_port=
    kotsadm_port=$(kubectl get svc -n kotsadm kotsadm -o jsonpath='{.spec.ports[?(@.name=="http")].port}')
    echo "kotsadm_port: $kotsadm_port"

    echo "fetching app versions"
    local versions=
    versions=$(curl -k -s -X GET "http://${kotsadm_ip}:${kotsadm_port}/api/v1/app/embedded-cluster-smoke-test-staging-app/versions?currentPage=0&pageSize=1" -H "Authorization: $kotsadm_auth_string")

    local total_count=
    total_count=$(echo "$versions" | jq -r '.totalCount')
    echo "totalCount: $total_count (expected: $expected_count)"

    if [ "$total_count" != "$expected_count" ]; then
        echo "expected $expected_count releases but got $total_count"
        echo "versions response: $versions"
        exit 1
    fi

    echo "release count matches expected: $expected_count"
}

main "$@"
