#!/bin/bash

./scripts/ci-build-deps.sh
./scripts/ci-build.sh
./scripts/ci-embed-release.sh
