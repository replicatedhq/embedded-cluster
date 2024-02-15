#!/usr/bin/env bash
set -euo pipefail


main() {
    local sha=
    sha="$1"

    apt-get update
    apt-get install curl ca-certificates -y

    curl "https://staging.replicated.app/embedded/embedded-cluster-smoke-test-staging-app/ci/${sha}" -H 'Authorization: 2cHePFvlGDksGOKfTicouqJKGtZ' -o ec-release.tgz
    tar xzf ec-release.tgz

    mv embedded-cluster-smoke-test-staging-app /usr/local/bin/embedded-cluster
    mv license.yaml /tmp/license.yaml
}
main "$@"
