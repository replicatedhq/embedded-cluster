#!/usr/bin/env bash
set -euox pipefail
PUPPETEER_EXECUTABLE_PATH=/snap/bin/chromium NODE_PATH="$(npm root -g)" "$@"
