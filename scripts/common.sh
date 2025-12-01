#!/bin/bash

OP_VAULT_NAME=${OP_VAULT_NAME:-"Developer Automation"}
OP_ITEM_NAME=${OP_ITEM_NAME:-"EC Dev"}

function log() {
    echo "$*" >&2
}

function fail() {
    log "error: $*"
    exit 1
}

function prefix_output() {
    local prefix=$1
    sed "s/^/[$prefix] /"
}

function require() {
    local var_name=${1:-}
    local var_value=${2:-}
    local is_secret=${3:-0}

    if [ -z "$var_value" ]; then
        fail "validation failed: $var_name unset"
    else
        if [ "$is_secret" == "1" ]; then
            log "$var_name is set to <secret>"
        else
            log "$var_name is set to $var_value"
        fi
    fi
}

function ensure_secret() {
    local env_var_name=$1
    local op_field_name=$2

    if [ -z "${!env_var_name:-}" ]; then
        secret=$(op_get_secret "$op_field_name")
        if [ -n "$secret" ]; then
            export "${env_var_name}=$secret"
        fi
    fi

    require "${env_var_name}" "${!env_var_name}" "1"
}

function op_get_secret() {
    local op_field_name=$1

    op item get "${OP_ITEM_NAME}" --vault "${OP_VAULT_NAME}" --fields "$op_field_name"
}

function ensure_current_user() {
    if [ -n "${CURRENT_USER:-}" ]; then
        return
    fi

    if [ -n "${GITHUB_USER:-}" ]; then
        CURRENT_USER="${GITHUB_USER}"
    else
        CURRENT_USER=$(id -u -n)
    fi

    export CURRENT_USER
}

function ensure_app_channel() {
    if [ -z "${APP_CHANNEL:-}" ]; then
        APP_CHANNEL="${CURRENT_USER}-dev"
    fi

    local channel_json
    channel_json=$(replicated channel ls -o json | jq -r ".[] | select(.name == \"${APP_CHANNEL}\")")
    if [ -z "$channel_json" ]; then
        replicated channel create --name "${APP_CHANNEL}" --description "EC dev automation" > /dev/null
    fi
    channel_json=$(replicated channel ls -o json | jq -r ".[] | select(.name == \"${APP_CHANNEL}\")")

    APP_CHANNEL=$(echo "$channel_json" | jq -r '.name')
    APP_CHANNEL_ID=$(echo "$channel_json" | jq -r '.id')
    APP_CHANNEL_SLUG=$(echo "$channel_json" | jq -r '.channelSlug')

    export APP_CHANNEL APP_CHANNEL_ID APP_CHANNEL_SLUG
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
