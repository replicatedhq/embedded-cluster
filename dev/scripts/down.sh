#!/bin/bash

set -e

. dev/scripts/common.sh

component=$1

# Check if a deployment name was provided
if [ -z "$component" ]; then
	echo "Error: No component name provided."
	exit 1
fi

# Check if already down
if [ ! -f "dev/patches/$component-down.yaml.tmp" ]; then
  echo "Error: already down."
  exit 1
fi

echo "Reverting..."

ec_exec k0s kubectl replace -f dev/patches/$component-down.yaml.tmp --force
ec_exec rm dev/patches/$component-down.yaml.tmp