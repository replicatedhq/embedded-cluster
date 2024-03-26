#!/usr/bin/env bash
set -euox pipefail

move_airgap() {
    # if an airgap file exists with pattern *.airgap, move it to /tmp/release.airgap
    for file in *.airgap;
    do
      if [ -e "$file" ]
      then
        mv "$file" /tmp/release.airgap
        break
      fi
    done

    # if there is no file at /tmp/release.airgap, this is an error
    if [ ! -e /tmp/release.airgap ]
    then
      echo "No airgap file found"
      exit 1
    fi
}

main() {
    local app_version_label=
    app_version_label="$1"
    local license_id=
    license_id="$2"
    local is_airgap=
    is_airgap="$3"

    apt-get update
    apt-get install curl ca-certificates -y

    echo "downloading from https://staging.replicated.app/embedded/embedded-cluster-smoke-test-staging-app/ci/${app_version_label}"
    curl "https://staging.replicated.app/embedded/embedded-cluster-smoke-test-staging-app/ci/${app_version_label}" -H "Authorization: ${license_id}" -o ec-release.tgz
    tar xzf ec-release.tgz

    mv embedded-cluster-smoke-test-staging-app /usr/local/bin/embedded-cluster
    mv license.yaml /tmp/license.yaml

    if [ "$is_airgap" = "true" ]; then
        move_airgap
    fi
}

main "$@"
