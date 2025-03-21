#!/usr/bin/env bash
set -euox pipefail

DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
. $DIR/common.sh

main() {
    if ! kubectl get cm -n kotsadm embedded-cluster-host-support-bundle -o yaml ; then
        echo "Failed to get configmap of remote host support bundle spec for kotsadm"
        return 1
    fi
}

main "$@"
