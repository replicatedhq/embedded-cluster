#!/bin/bash

main() {
    chart="$1"
    /usr/local/bin/install_helm.sh
    helm pull "$chart"
}

main "$@"
