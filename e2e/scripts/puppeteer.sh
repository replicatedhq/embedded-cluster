#!/usr/bin/env bash
set -euo pipefail
PUPPETEER_SKIP_DOWNLOAD=1 NODE_PATH="$(npm root -g)" "$@"
