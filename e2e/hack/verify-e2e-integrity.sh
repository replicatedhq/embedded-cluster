#!/usr/bin/env bash
if ! git diff --exit-code; then
    echo "make create-e2e-workflows introduced changes"
    exit 1
fi
newfiles=$(git ls-files --others --exclude-standard)
if [ -n  "$newfiles" ]; then
    echo "make create-e2e-workflows introduced new files:"
    echo "$newfiles"
    exit 1
fi
