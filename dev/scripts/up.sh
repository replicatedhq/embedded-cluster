#!/bin/bash

set -e

. dev/scripts/common.sh

component=$1

# Check if a component name was provided
if [ -z "$component" ]; then
	echo "Error: No component name provided."
	exit 1
fi

# Check if already up
if [ -f "dev/patches/$component-down.yaml.tmp" ]; then
  ec_up $component
  exit 0
fi

# Save current deployment state
ec_exec k0s kubectl get deployment $(deployment $component) -n embedded-cluster -oyaml > dev/patches/$component-down.yaml.tmp

# Patch the deployment
ec_patch $component

# Wait for rollout to complete
ec_exec k0s kubectl rollout status deployment/$(deployment $component) -n embedded-cluster

# Up into the updated deployment
ec_up $component
