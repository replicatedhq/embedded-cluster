#!/bin/bash

set -euo pipefail

# This script applies k0s version-specific patches to the codebase
# Usage: ./scripts/apply-k0s-patches.sh <k0s_major_minor>
# Example: ./scripts/apply-k0s-patches.sh 1.29

K0S_MAJOR_MINOR=${1:-}
echo "Applying patches for k0s version $K0S_MAJOR_MINOR"

PATCH_DIR="patches/k0s-$K0S_MAJOR_MINOR"

if [[ ! -d "$PATCH_DIR" ]]; then
  echo "No patches directory found for k0s $K0S_MAJOR_MINOR at $PATCH_DIR"
  exit 1
fi

# Count the number of patches
PATCH_COUNT=$(find "$PATCH_DIR" -name "*.patch" 2>/dev/null | wc -l | tr -d ' ')
if [[ "$PATCH_COUNT" -eq 0 ]]; then
  echo "No patches found in $PATCH_DIR"
  exit 1
fi

echo "Found $PATCH_COUNT patches in $PATCH_DIR"

# Apply patches in order
for PATCH in $(find "$PATCH_DIR" -name "*.patch" | sort); do
  echo "Applying patch: $(basename "$PATCH")"
  if ! git apply --whitespace=fix "$PATCH"; then
    echo "Failed to apply patch: $PATCH"
    exit 1
  fi
done

echo "All patches for k0s $K0S_MAJOR_MINOR applied successfully"

# Update go.mod and go.sum
echo "Running 'make go.mod' to update dependencies"
make go.mod

echo "Patch process completed successfully"
