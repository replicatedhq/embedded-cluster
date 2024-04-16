#!/usr/bin/env bash
set -euox pipefail

main() {
    tar xzf /tmp/ec-release.tgz

    mv embedded-cluster-smoke-test-staging-app /usr/local/bin/embedded-cluster
    mv license.yaml /tmp/license.yaml

    for file in *.airgap;
    do
      if [ -e "$file" ]
      then
        mv "$file" /tmp/release.airgap
        break
      fi
    done

    # if there is no file at /tmp/release.airgap, this is an error
    if [ ! -e /tmp/release.airgap ]
    then
      echo "No airgap file found"
      exit 1
    fi

    rm /tmp/ec-release.tgz
}

main "$@"
