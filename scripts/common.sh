#!/bin/bash

function log() {
    echo "$*" >&2
}

function fail() {
    log "error: $*"
    exit 1
}

function require() {
    if [ -z "$2" ]; then
        fail "validation failed: $1 unset"
    else
        log "$1 is set to $2"
    fi
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
            log "retry $count/$retries exited $exit, retrying in $wait seconds..."
            sleep $wait
        else
            log "retry $count/$retries exited $exit, no more retries left"
            return $exit
        fi
    done
    return 0
}

function url_encode_semver() {
    echo "${1//+/%2B}"
}
