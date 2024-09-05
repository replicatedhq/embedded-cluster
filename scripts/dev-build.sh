#!/bin/bash

set -euo pipefail

./scripts/ci-build-deps.sh
./scripts/ci-build.sh
./scripts/ci-embed-release.sh
