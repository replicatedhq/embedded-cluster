#!/bin/bash

set -euox pipefail

# we're only using the APKINDEX files to get the versions, so it doesn't matter which arch we use

mkdir -p output/tmp
curl -L -o output/tmp/APKINDEX.tar.gz https://packages.wolfi.dev/os/x86_64/APKINDEX.tar.gz
tar -xzvf output/tmp/APKINDEX.tar.gz -C output/tmp

k0s_version=${1-}
if [ -n "$k0s_version" ]; then
  make pkg/goods/bins/k0s K0S_VERSION="$k0s_version" K0S_BINARY_SOURCE_OVERRIDE=
else
  make pkg/goods/bins/k0s
fi

function get_package_version() {
  # get the version specified by k0s
  pinned_version=$(pkg/goods/bins/k0s airgap list-images --all | grep "/$1:" | awk -F':' '{ print $2 }' | sed 's/^v//' | sed 's/-[0-9]*$//')
  # find the corresponding APK package version
  < output/tmp/APKINDEX grep -A1 "^P:$1" | grep "V:$pinned_version" | awk -F '-r' '{print $1, $2}' | sort -k2,2n | tail -1 | awk '{print $1 "-r" $2}' | sed -n -e 's/V://p' | tr -d '\n'
}

components='[
  {
    "name": "coredns",
    "version": "'$(get_package_version coredns)'",
    "makefile_var": "COREDNS_VERSION"
  },
  {
    "name": "calico-node",
    "version": "'$(get_package_version calico-node)'",
    "makefile_var": "CALICO_NODE_VERSION"
  },
  {
    "name": "metrics-server",
    "version": "'$(get_package_version metrics-server)'",
    "makefile_var": "METRICS_SERVER_VERSION"
  }
]'

make bin/apko

for component in $(echo "${components}" | jq -c '.[]'); do
  name=$(echo "$component" | jq -r '.name')
  version=$(echo "$component" | jq -r '.version')
  makefile_var=$(echo "$component" | jq -r '.makefile_var')

  sed "s/__VERSION__/$version/g" deploy/images/"$name"/apko.tmpl.yaml > output/tmp/apko.yaml

  make apko-build-and-publish \
    IMAGE=ttl.sh/ec/"$name":"$version" \
    APKO_CONFIG=output/tmp/apko.yaml \
    VERSION="$version"

  digest=$(awk -F'@' '{print $2}' build/digest)
  sed -i "s/^$makefile_var.*/$makefile_var = $version@$digest/" Makefile
done
