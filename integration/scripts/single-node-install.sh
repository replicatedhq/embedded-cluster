#!/bin/bash

main() {
    if ! /usr/local/bin/helmvm install --no-prompt >/tmp/log 2>&1; then
        cat /tmp/log
        echo "Failed to install helmvm"
        exit 1
    fi
    if ! grep -q "You can now access your cluster" /tmp/log; then
        cat /tmp/log
        echo "Failed to install helmvm"
        exit 1
    fi
    /usr/local/bin/wait-for-ready-nodes.sh 1 >> /tmp/log
    cat /tmp/log
}

main
