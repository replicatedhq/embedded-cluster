#!/bin/bash

echo "
[Unit]
Description=Set default route through proxy node
After=network-online.targej
Wants=network-online.target

[Service]
Type=oneshot
ExecStartPre=/bin/sleep 5
ExecStart=/bin/bash -c 'ip route del default; ip route add default via 10.0.0.254'
RemainAfterExit=true

[Install]
WantedBy=multi-user.target" > /etc/systemd/system/default-route-through-proxy.service

systemctl daemon-reload
systemctl enable default-route-through-proxy
systemctl start default-route-through-proxy
