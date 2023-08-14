#!/bin/bash

main() {
    chart="$1"
    output="$2"
    /usr/local/bin/pull-helm-chart.sh "$chart"

    chart="$1"
    helmvm embed --chart "$chart" --output embed
}

main "$@"
