#!/usr/bin/env bash
set -euox pipefail

DIR=/usr/local/bin
. $DIR/common.sh

main() {
    echo "upgrading from airgap bundle"
    embedded-cluster-upgrade update --airgap-bundle /assets/upgrade/release.airgap
}

main "$@"
