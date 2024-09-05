#!/bin/bash

set -euo pipefail

export EC_VERSION APP_VERSION
EC_VERSION="$(git describe --tags --match='[0-9]*.[0-9]*.[0-9]*')"
APP_VERSION="appver-dev-$(git rev-parse --short HEAD)"

./scripts/ci-build-deps.sh
./scripts/ci-build.sh
./scripts/ci-embed-release.sh
