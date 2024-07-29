#!/bin/bash

set -e

if [ "$#" -ne 1 ] || [ "$1" == "" ] || [ "$1" == "-h" ] || [ "$1" == "--help" ]; then
    echo "Usage: $0 <image_id>"
    exit 1
fi

image_id="$1"
image_id=$(echo "$image_id" | cut -d'@' -f1) # remove digest
# make sure if there is only one colon it is not the port
if ! echo "$image_id" | rev | cut -d':' -f1 | rev | grep -q '/' ; then
    image_id=$(echo "$image_id" | rev | cut -d':' -f2- | rev) # remove tag
fi

echo -n "$image_id"
