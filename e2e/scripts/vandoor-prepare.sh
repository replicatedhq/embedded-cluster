#!/usr/bin/env bash
set -euox pipefail

main() {
    local app_version_label=
    app_version_label="$1"
    local license_id=
    license_id="$2"

    echo "downloading from https://staging.replicated.app/embedded/embedded-cluster-smoke-test-staging-app/ci/${app_version_label}"
    retry 5 curl --retry 5 --retry-all-errors -fL -o ec-release.tgz "https://staging.replicated.app/embedded/embedded-cluster-smoke-test-staging-app/ci/${app_version_label}" -H "Authorization: ${license_id}"
    tar xzf ec-release.tgz

    mkdir -p /assets
    mv embedded-cluster-smoke-test-staging-app /usr/local/bin/embedded-cluster
    mv license.yaml /assets/license.yaml
}

main "$@"
