#!/usr/bin/env bash
set -euox pipefail

DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
. $DIR/common.sh

main() {
    echo "upgrading from airgap bundle"
    embedded-cluster-upgrade update --airgap-bundle /assets/upgrade/release.airgap
}

main "$@"
