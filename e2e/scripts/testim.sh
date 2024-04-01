#!/usr/bin/env bash
set -euox pipefail

main() {
  local testim_token="$1"
  if [ -z "$testim_token" ]; then
    echo "Testim token is required"
    exit 1
  fi

  local testim_branch="$2"
  if [ -z "$testim_branch" ]; then
    echo "Testim branch is required"
    exit 1
  fi

  local test_name="$3"
  if [ -z "$test_name" ]; then
    echo "Test name is required"
    exit 1
  fi

  echo "Running Testim test: $test_name on branch $testim_branch"

  # port-forward to kotsadm to bypass kurl-proxy
  kubectl port-forward sts/kotsadm -n kotsadm 3000:3000 &
  port_forward_pid=$!

  # run the Testim test
  # TODO: change project to Embedded Cluster project once it's created
  testim --token=$testim_token --project=wpYAooUimFDgQxY73r17 --grid=Testim-grid --branch=$testim_branch --timeout=3600000 --name=$test_name --tunnel --tunnel-port=3000

  # kill the port-forward process
  kill $port_forward_pid

  echo "Testim test $test_name completed"
}

export KUBECONFIG=/var/lib/k0s/pki/admin.conf
export PATH=$PATH:/var/lib/embedded-cluster/bin
main "$@"
