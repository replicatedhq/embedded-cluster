#!/usr/bin/env bash
# This scripts runs helmvm's build-bundle command.
set -euo pipefail

main() {
    if ! helmvm build-bundle ; then
        echo "Failed to build bundle"
        exit 1
    fi
    if ! ls bundle/base_images.tar; then
        echo "Unable to find bundle/base_images.tar"
        ls -la bundle
        exit 1
    fi
}

main
