#!/usr/bin/env bash
set -euox pipefail

DIR=/usr/local/bin
. $DIR/common.sh

function collect_support_bundle() {
    "${EMBEDDED_CLUSTER_BASE_DIR}/bin/kubectl-support_bundle" --output host.tar.gz --interactive=false "${EMBEDDED_CLUSTER_BASE_DIR}/support/host-support-bundle.yaml"
}

function collect_installer_logs() {
    # Collect logs from V2 (/var/log/embedded-cluster) or V3 (/var/log/{appslug}) path
    # based on ENABLE_V3 environment variable
    local log_dir

    if [ "${ENABLE_V3}" = "1" ]; then
        # V3: Use app slug directory
        log_dir="/var/log/${APP_SLUG}"
    else
        # V2: Use static embedded-cluster directory
        log_dir="/var/log/embedded-cluster"
    fi

    if [ -d "${log_dir}" ]; then
        tar -czvf host.tar.gz "${log_dir}" 2>/dev/null || true
    fi
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
