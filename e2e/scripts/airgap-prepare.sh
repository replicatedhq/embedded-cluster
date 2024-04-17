#!/usr/bin/env bash
set -euox pipefail

main() {
    tar xzf /tmp/ec-release.tgz

    mv embedded-cluster-smoke-test-staging-app /usr/local/bin/embedded-cluster
    mv license.yaml /tmp/license.yaml
    mv /tmp/kubectl-kots /usr/local/bin/kubectl-kots

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

    if [ -e /tmp/ec-release-upgrade.tgz ]
    then
      mkdir -p upgrade
      tar xzf /tmp/ec-release-upgrade.tgz -C upgrade

      mv upgrade/embedded-cluster-smoke-test-staging-app /usr/local/bin/embedded-cluster
      mv upgrade/license.yaml /tmp/license.yaml

      for file in upgrade/*.airgap;
      do
        if [ -e "$file" ]
        then
          mv "$file" /tmp/release-upgrade.airgap
          break
        fi
      done

      # if there is no file at /tmp/release-upgrade.airgap, this is an error
      if [ ! -e /tmp/release-upgrade.airgap ]
      then
        echo "No upgrade airgap file found"
        exit 1
      fi
    fi
}

main "$@"
