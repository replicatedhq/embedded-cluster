#!/bin/bash

set -euo pipefail

# Detect OS and use appropriate sed syntax
if [[ "$OSTYPE" == "darwin"* ]]; then
    SED_ARGS=(-i '')
else
    SED_ARGS=(-i)
fi

function update_k0s_minor_version() {
    local minor_version=$1
    local k0s_version

    k0s_version=$(gh release list --limit 100 --repo k0sproject/k0s --exclude-pre-releases --json name,isLatest | \
        jq -r "[.[] | select(.name | startswith(\"v1.$minor_version\"))] | first | .name")

    if [[ "$k0s_version" == "null" ]]; then
        echo "No k0s version found for v1.$minor_version"
        exit 1
    fi

    sed "${SED_ARGS[@]}" "/^K0S_VERSION_1_${minor_version} = .*/d" Makefile
    # Use a more portable approach that works on both Linux and macOS
    if [[ "$OSTYPE" == "darwin"* ]]; then
        # macOS sed needs explicit newline
        sed "${SED_ARGS[@]}" "/# +++ Start K0S Versions +++/a\\
K0S_VERSION_1_${minor_version} = $k0s_version\\
" Makefile
    else
        # Linux sed automatically adds newline, so we don't need the trailing backslash
        sed "${SED_ARGS[@]}" "/# +++ Start K0S Versions +++/a\\
K0S_VERSION_1_${minor_version} = $k0s_version" Makefile
    fi
}

function main() {
    local minor_version=$1
    local minor_version_minus_1=$((minor_version - 1))
    local minor_version_minus_2=$((minor_version - 2))
    local minor_version_minus_3=$((minor_version - 3))

    update_k0s_minor_version "$minor_version_minus_3"
    update_k0s_minor_version "$minor_version_minus_2"
    update_k0s_minor_version "$minor_version_minus_1"
    update_k0s_minor_version "$minor_version"

    # pin to the current major.minor version
    sed "${SED_ARGS[@]}" "s/^K0S_MINOR_VERSION = .*$/K0S_MINOR_VERSION = $minor_version/" Makefile

    # substitute images for the major.minor version minus 2
    ./scripts/k0s-build-minor-version.sh "$minor_version_minus_2"

    # reset go.mod and go.sum
    git checkout -- **/go.mod **/go.sum

    # substitute images for the major.minor version minus 1
    ./scripts/k0s-build-minor-version.sh "$minor_version_minus_1"

    # reset go.mod and go.sum
    git checkout -- **/go.mod **/go.sum

    # prepare the code for the current major.minor version
    ./scripts/k0s-build-minor-version.sh "$minor_version"

    echo "Done"
}

main "$@"
