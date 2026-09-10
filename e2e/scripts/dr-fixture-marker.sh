#!/usr/bin/env bash
set -euo pipefail

readonly marker=embedded-cluster-dr-fixture-v1
readonly marker_path=/var/lib/dr-fixture/marker

case "${1:-}" in
write)
    kubectl exec -n kotsadm deployment/nginx -- \
        sh -eu -c 'printf "%s\n" "$1" > "$2"' -- "$marker" "$marker_path"
    ;;
read)
    kubectl exec -n kotsadm deployment/nginx -- cat "$marker_path"
    ;;
*)
    echo "usage: $0 write|read" >&2
    exit 2
    ;;
esac
