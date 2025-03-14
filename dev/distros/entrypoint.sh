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
    tr -dc 'a-z0-9' < /dev/urandom | head -c32 > /etc/machine-id.persistent
fi
mkdir -p /var/lib/dbus
ln -sf /etc/machine-id.persistent /etc/machine-id
ln -sf /etc/machine-id.persistent /var/lib/dbus/machine-id

# Override timesyncd config to allow it to run in containers
mkdir -p /etc/systemd/system/systemd-timesyncd.service.d/
cat > /etc/systemd/system/systemd-timesyncd.service.d/override.conf << EOF
[Unit]
ConditionVirtualization=
EOF

# Wait for systemd then start timesyncd
(
    until systemctl is-system-running -q; do
        sleep 1
    done
    systemctl enable systemd-timesyncd
    systemctl start systemd-timesyncd
) &

# Launch the system
exec /sbin/init
