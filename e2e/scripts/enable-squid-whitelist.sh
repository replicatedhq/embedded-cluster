#!/bin/bash
set -euxo pipefail

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

function retry() {
    local retries=$1
    shift

    local count=0
    until "$@"; do
        exit=$?
        wait=$((2 ** count))
        count=$((count + 1))
        if [ $count -lt "$retries" ]; then
            sleep $wait
        else
            return $exit
        fi
    done
    return 0
}

function validate_whitelist() {
    # validate that we can access ec-e2e-replicated-app.testcluster.net
    status_code=$(curl -s -o /dev/null -w "%{http_connect}" -x http://10.0.0.254:3128 https://ec-e2e-replicated-app.testcluster.net/market/v1/echo/ip)
    if [ "$status_code" -ne 200 ]; then
        echo "Error: ec-e2e-replicated-app.testcluster.net expected status code 200, got $status_code"
        return 1
    fi

    # validate that we cannot access google.com (should be blocked)
    status_code=$(curl -s -o /dev/null -w "%{http_connect}" -x http://10.0.0.254:3128 https://google.com)
    if [ "$status_code" -ne 403 ] && [ "$status_code" -ne 407 ]; then
        echo "Error: google.com expected status code 403 or 407 (blocked), got $status_code"
        return 1
    fi

    return 0
}

function main() {
    # install curl if it's not already installed
    maybe_install curl curl

    # update the squid config to disable allow all
    sed -i 's/http_access allow all/# http_access allow all/' /etc/squid/conf.d/ec.conf

    # restart the squid service
    squid -k reconfigure

    # wait a bit for squid to be ready
    retry 5 validate_whitelist
}

main "$@"
