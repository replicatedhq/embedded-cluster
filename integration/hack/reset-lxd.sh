#!/bin/bash

# This script stops all running containers, deletes all containers, networks,
# and profiles that were created by the tests.

# Stop all containers
for container in  $(lxc list -c n --format csv | grep "node-"); do
    echo "Stopping container: $container"
    lxc stop "$container"
done

# Delete all containers
for container in  $(lxc list -c n --format csv | grep "node-"); do
    echo "Deleting container: $container"
    lxc rm "$container"
done

# Delete all internal networks
for network in $(lxc network list -f csv | cut -d, -f1 | grep "internal-"); do
    echo "Deleting internal network: $network"
    lxc network delete "$network"
done

# Delete all external networks
for network in $(lxc network list -f csv | cut -d, -f1 | grep "external-"); do
    echo "Deleting network: $network"
    lxc network delete "$network"
done

# Delete all profiles 
for profile in $(lxc profile list --format csv | cut -d, -f1 | grep "profile-"); do
    echo "Deleting profile: $profile"
    lxc profile delete "$profile"
done
