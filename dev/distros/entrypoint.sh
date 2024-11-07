#!/bin/bash

# Disable unneeded services
systemctl mask \
    apt-daily-upgrade.service \
    apt-daily.service \
    dpkg-db-backup.service \
    getty-static.service \
    getty@tty1.service \
    console-getty.service \
    systemd-firstboot.service \
    systemd-ask-password-console.service \
    systemd-ask-password-wall.service \
    emergency.service \
    rescue.service

# A unique machine ID is required for multi-node clusters in k0s <= v1.29
# https://github.com/k0sproject/k0s/blob/443e28b75d216e120764136b4513e6237cea7cc5/docs/external-runtime-deps.md#a-unique-machine-id-for-multi-node-setups
if [ ! -f "/etc/machine-id.persistent" ]; then
    dbus-uuidgen --ensure=/etc/machine-id.persistent
fi
ln -sf /etc/machine-id.persistent /etc/machine-id
ln -sf /etc/machine-id.persistent /var/lib/dbus/machine-id

# Sync time
chronyc -a makestep

# Launch the system
exec /sbin/init
