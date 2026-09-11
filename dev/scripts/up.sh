#!/bin/bash

set -e

. dev/scripts/common.sh

component=$1

# Check if a component name was provided
if [ -z "$component" ]; then
	echo "Error: No component name provided."
	exit 1
fi

# Ensure dev go cache / go mod cache directories exists
mkdir -p dev/build/.gocache dev/build/.gomodcache

# Build and load the image into the embedded cluster
ec_build_and_load "$component"

# Save original state
if [ ! -f "dev/patches/$component-down.yaml.tmp" ]; then
  ec_exec k0s kubectl get deployment $(deployment $component) -n embedded-cluster -oyaml > dev/patches/$component-down.yaml.tmp
fi

# Patch the deployment
ec_patch $component

# Wait for rollout to complete
ec_exec k0s kubectl rollout status deployment/$(deployment $component) -n embedded-cluster

# Up into the updated deployment
ec_up $component
