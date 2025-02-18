#!/usr/bin/env bash
set -euxo pipefail

systemctl enable --now firewalld

firewall-cmd --set-default-zone=drop
firewall-cmd --reload
