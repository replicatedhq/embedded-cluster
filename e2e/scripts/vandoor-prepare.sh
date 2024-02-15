#!/usr/bin/env bash
set -euo pipefail


main() {
    local sha=
    sha="$1"

    curl "https://staging.replicated.app/embedded/embedded-cluster-smoke-test-staging-app/ci/${sha}" -H 'Authorization: 2cHePFvlGDksGOKfTicouqJKGtZ' -o ec-release.tgz
    tar xzf ec-release.tgz

    mv embedded-cluster-smoke-test-staging-app embedded-cluster
}
main "$@"
