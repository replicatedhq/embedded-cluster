#!/bin/bash

set -euo pipefail

required_env_vars=(
  CHART_VERSION
  IMAGE_NAME
  IMAGE_TAG
  CHART_REMOTE
)

for var in "${required_env_vars[@]}"; do
  if [ -z "${!var:-}" ]; then
    echo "Error: $var is not set"
    exit 1
  fi
done

export CHART_VERSION
export IMAGE_NAME
export IMAGE_TAG

envsubst < Chart.yaml.tmpl > Chart.yaml
envsubst < values.yaml.tmpl > values.yaml

CHART_NAME="$(helm package . | rev | cut -d/ -f1 | rev)"

echo "pushing $CHART_NAME to $CHART_REMOTE"
if [ -z "${HELM_USER:-}" ] || [ -z "${HELM_PASS:-}" ]; then
  echo "HELM_USER or HELM_PASS not set, skipping helm login"
else
  helm registry login "$HELM_REGISTRY" --username "$HELM_USER" --password "$HELM_PASS"
fi
helm push "$CHART_NAME" "$CHART_REMOTE"
