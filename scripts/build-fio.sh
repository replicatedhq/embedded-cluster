#!/bin/bash

set -euo pipefail

version=${1:?fio version is required}
arch=${2:?target architecture is required}
output=${3:?output path is required}

host_arch=$(go env GOARCH)
if [ "$arch" != "$host_arch" ]; then
    echo "fio must be built on a $arch runner (this runner is $host_arch)" >&2
    exit 1
fi

workdir=$(mktemp -d "${TMPDIR:-/tmp}/build-fio.XXXXXX")
trap 'rm -rf "$workdir"' EXIT

curl --retry 5 --retry-all-errors -fsSL \
    "https://api.github.com/repos/axboe/fio/tarball/fio-$version" \
    -o "$workdir/fio.tar.gz"
tar -xzf "$workdir/fio.tar.gz" --strip-components=1 -C "$workdir"

(
    cd "$workdir"
    ./configure --build-static --disable-native
    make -j"$(getconf _NPROCESSORS_ONLN)"
)

cp "$workdir/fio" "$output"
