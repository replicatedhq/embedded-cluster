#!/usr/bin/env bash
set -euo pipefail
PUPPETEER_EXECUTABLE_PATH=/snap/bin/chromium NODE_PATH="$(npm root -g)" "$@"
