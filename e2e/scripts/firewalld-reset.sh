#!/bin/bash

set -euxo pipefail

embedded-cluster reset firewalld

# check if firewalld is reset
if firewall-cmd --info-zone ec-net >/dev/null 2>&1 ; then
    echo "firewalld is not reset, ec-net zone exists"
    firewall-cmd --info-zone ec-net
    exit 1
fi

if firewall-cmd --list-all | grep -q 10250 ; then
    echo "firewalld is not reset, 10250 port is open"
    firewall-cmd --list-all
    exit 1
fi

echo "firewalld is reset"
