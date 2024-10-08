#!/bin/bash

# Disable getty service as it's flaky and doesn't apply in containers
systemctl mask getty@tty1.service

# A unique machine ID is required for multi-node clusters in k0s <= v1.29
# https://github.com/k0sproject/k0s/blob/443e28b75d216e120764136b4513e6237cea7cc5/docs/external-runtime-deps.md#a-unique-machine-id-for-multi-node-setups
if [ ! -f "/etc/machine-id.persistent" ]; then
    dbus-uuidgen --ensure=/etc/machine-id.persistent
fi
ln -sf /etc/machine-id.persistent /etc/machine-id
ln -sf /etc/machine-id.persistent /var/lib/dbus/machine-id

# Launch the system
exec /sbin/init
