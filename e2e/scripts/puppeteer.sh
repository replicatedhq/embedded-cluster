#!/usr/bin/env bash
set -euo pipefail
NODE_PATH="$(npm root -g)" "$@"
