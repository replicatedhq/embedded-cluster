#!/bin/bash

# This scripts takes an EC version and a k0s minor version and substitutes the k0s version in the EC version
# and outputs the new EC version.

set -euo pipefail

function main() {
    local ec_version="$1"
    local k0s_minor_version="$2"

    # substitute the k0s version in the EC version
    echo "$ec_version" | sed -E "s/^([0-9]+\.[0-9]+\.[0-9]+)\+k8s-[0-9]+\.[0-9]+(.*)$/\1+k8s-1.$k0s_minor_version\2/"
}


main "$@"
