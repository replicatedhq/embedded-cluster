#!/usr/bin/env bash
set -euox pipefail

main() {
  if [ -z "$1" ]; then
    echo "Test name is required"
    exit 1
  fi
  local test_name="$1"

  if [ "$test_name" == "create-backup" ]; then
    export DR_AWS_S3_ENDPOINT="$2"
    export DR_AWS_S3_REGION="$3"
    export DR_AWS_S3_BUCKET="$4"
    export DR_AWS_S3_PREFIX="$5"
    export DR_AWS_ACCESS_KEY_ID="$6"
    export DR_AWS_SECRET_ACCESS_KEY="$7"
  fi

  export BASE_URL="http://10.0.0.2:30001"
  cd /tmp/playwright
  npx playwright test "$test_name"
}

main "$@"
