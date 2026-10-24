#!/usr/bin/env bash
set -euo pipefail

DIR=/usr/local/bin
. "$DIR/common.sh"

kubectl -n kotsadm get deployment kotsadm \
  -o 'jsonpath={.spec.template.spec.containers[0].image}'
