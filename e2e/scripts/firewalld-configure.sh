#!/bin/bash

set -euxo pipefail

function setup_enterprise_firewall() {
    ## Deny all traffic by default and only allow specific services

    # Set default zone to drop
    sudo firewall-cmd --set-default-zone=drop

    ## GCP specific

    # Allow tunnel through IAP
    sudo firewall-cmd --permanent --new-zone=gcp
    sudo firewall-cmd --permanent --zone=gcp --add-source=35.235.240.0/20
    sudo firewall-cmd --permanent --zone=gcp --add-service=ssh

    # Set up logging for rejected packets
    sudo firewall-cmd --permanent --direct --add-rule ipv4 filter INPUT 0 -m limit --limit 5/min -j LOG --log-prefix "Dropped INPUT: "

    ## Apply the changes
    sudo firewall-cmd --reload
}

function configure_firewall() {
    # Configure host network
    # Allow other nodes to connect to k0s core components
    sudo firewall-cmd --permanent --add-port=6443/tcp # apiserver
    sudo firewall-cmd --permanent --add-port=10250/tcp # kubelet
    sudo firewall-cmd --permanent --add-port=9443/tcp # k0s api
    sudo firewall-cmd --permanent --add-port=2380/tcp # etcd
    sudo firewall-cmd --permanent --add-port=4789/udp # calico

    # Configure pod and service networks
    sudo firewall-cmd --permanent --new-zone=ec-net
    # Set the default target to ACCEPT
    sudo firewall-cmd --permanent --zone=ec-net --set-target=ACCEPT
    # Add the pod and service networks
    sudo firewall-cmd --permanent --zone=ec-net --add-source="$POD_NETWORK"
    sudo firewall-cmd --permanent --zone=ec-net --add-source="$SERVICE_NETWORK"
    # Add the calico interfaces
    # This is redundant and overlaps with the pod network but we add it anyway
    sudo firewall-cmd --permanent --zone=ec-net --add-interface="cali+"
    sudo firewall-cmd --permanent --zone=ec-net --add-interface="tunl+"
    sudo firewall-cmd --permanent --zone=ec-net --add-interface="vxlan-v6.calico"
    sudo firewall-cmd --permanent --zone=ec-net --add-interface="vxlan.calico"
    sudo firewall-cmd --permanent --zone=ec-net --add-interface="wg-v6.cali"
    sudo firewall-cmd --permanent --zone=ec-net --add-interface="wireguard.cali"

    ## Apply the changes
    sudo firewall-cmd --reload
}

function main() {
    setup_enterprise_firewall
    configure_firewall
}

POD_NETWORK="10.244.0.0/17"
SERVICE_NETWORK="10.244.128.0/17"

main "$@"







sudo firewall-cmd --permanent --zone=ec-pods --add-interface=eth0
sudo firewall-cmd --permanent --zone=ec-pods --add-masquerade
sudo firewall-cmd --reload