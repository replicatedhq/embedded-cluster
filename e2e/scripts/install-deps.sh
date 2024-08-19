#!/usr/bin/env bash
set -euo pipefail

function maybe_install() {
    local package=$1
    local command=$2

    if [ -n "$command" ]; then
        if command -v "$command" ; then
            echo "$command is already installed"
            return
        fi
    fi

    if command -v apt-get ; then
        apt_install "$package"
    elif command -v yum ; then
        yum_install "$package"
    else
        echo "Unsupported package manager"
        exit 1
    fi
}

function apt_install() {
    local package=$1

    apt-get update
    apt-get install -y "$package"
}

function yum_install() {
    local package=$1

    yum install -y "$package"
}

function main() {
    if command -v apt-get ; then
        echo "Installing apt-utils"
        apt_install apt-utils
    fi
    echo "Installing modprobe"
    maybe_install kmod modprobe
    echo "Installing chronyd"
    maybe_install chrony chronyd
}

main
