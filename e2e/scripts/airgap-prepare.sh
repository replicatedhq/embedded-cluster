#!/usr/bin/env bash
set -euox pipefail

main() {
    tar xzf /assets/ec-release.tgz

    mv embedded-cluster-smoke-test-staging-app /usr/local/bin/embedded-cluster
    mv license.yaml /assets/license.yaml

    for file in *.airgap;
    do
      if [ -e "$file" ]
      then
        mv "$file" /assets/release.airgap
        break
      fi
    done

    # delete the ec airgap release
    rm /assets/ec-release.tgz

    # if there is no file at /assets/release.airgap, this is an error
    if [ ! -e /assets/release.airgap ]
    then
      echo "No airgap file found"
      exit 1
    fi

    if [ -e /assets/ec-release-upgrade.tgz ]
    then
      mkdir -p upgrade
      tar xzf /assets/ec-release-upgrade.tgz -C upgrade

      mv upgrade/embedded-cluster-smoke-test-staging-app /usr/local/bin/embedded-cluster-upgrade
      mkdir -p /assets/upgrade
      mv upgrade/license.yaml /assets/upgrade/license.yaml

      for file in upgrade/*.airgap;
      do
        if [ -e "$file" ]
        then
          mv "$file" /assets/upgrade/release.airgap
          break
        fi
      done

      # if there is no file at /assets/upgrade/release.airgap, this is an error
      if [ ! -e /assets/upgrade/release.airgap ]
      then
        echo "No upgrade airgap file found"
        exit 1
      fi

      # delete the ec upgrade airgap release
      rm /assets/ec-release-upgrade.tgz
    fi
}

main "$@"
