#!/usr/bin/env bash
set -euox pipefail

main() {
  if [ -z "$1" ]; then
    echo "Testim token is required"
    exit 1
  fi
  local testim_token="$1"

  if [ -z "$2" ]; then
    echo "Testim branch is required"
    exit 1
  fi
  local testim_branch="$2"

  if [ -z "$3" ]; then
    echo "Test name is required"
    exit 1
  fi
  local test_name="$3"

  echo "Running Testim test: $test_name on branch $testim_branch"

  # testim CLI can only tunnel to localhost, so this allows us to forward to the desired local address
  socat TCP-LISTEN:3000,fork TCP:10.0.0.2:30001 &
  socat_pid=$!

  sleep 5

  # run the Testim test
  testim --token=$testim_token --project=wSvaGXFJnnoonKzLxBfX --grid=Testim-grid --branch=$testim_branch --timeout=3600000 --name=$test_name --tunnel --tunnel-port=3000

  kill $socat_pid

  echo "Testim test $test_name completed"
}

main "$@"
