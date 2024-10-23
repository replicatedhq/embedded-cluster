#!/usr/bin/env bash
set -euox pipefail

main() {
    local directory="$1"
    if ! [[ -d "$directory" ]]; then
        echo "Directory $directory does not exist"
        exit 0
    fi

    if [[ -z "$(ls -A "$directory")" ]]; then
        echo "Directory $directory is empty"
        exit 0
    fi

    echo "Directory $directory is not empty"
    ls -A "$directory"
    exit 1
}

main "$@"
