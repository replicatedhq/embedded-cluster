#!/bin/bash

set -euxo pipefail

# check if firewalld is configured correctly
if ! firewall-cmd --info-zone ec-net >/dev/null 2>&1 ; then
    echo "firewalld is not configured correctly, ec-net zone does not exist"
    firewall-cmd --list-all-zones
    exit 1
fi

if ! firewall-cmd --list-all | grep -q 10250 ; then
    echo "firewalld is not configured correctly, 10250 port is not open"
    firewall-cmd --list-all
    exit 1
fi

echo "firewalld validated"
