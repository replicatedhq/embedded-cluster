#!/usr/bin/env bash
set -euox pipefail

DIR=/usr/local/bin
. $DIR/common.sh

function collect_support_bundle() {
    export TROUBLESHOOT_AUTO_UPDATE=false
    "${EMBEDDED_CLUSTER_BASE_DIR}/bin/kubectl-support_bundle" --output host.tar.gz --interactive=false "${EMBEDDED_CLUSTER_BASE_DIR}/support/host-support-bundle.yaml"
}

function collect_installer_logs() {
    tar -czvf host.tar.gz /var/log/embedded-cluster
}

main() {
    if ! collect_support_bundle; then
        echo "Failed to collect support bundle"
        if ! collect_installer_logs; then
            echo "Failed to collect installer logs"
            return 1
        fi
        echo "Installer logs collected successfully"
        return 0
    fi

    echo "Support bundle collected successfully"
}

main "$@"
