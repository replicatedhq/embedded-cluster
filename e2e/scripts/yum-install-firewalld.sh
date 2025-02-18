#!/usr/bin/env bash
set -euxo pipefail

yum update -y
yum install -y firewalld
systemctl enable --now firewalld

firewall-cmd --set-default-zone=drop
firewall-cmd --reload
