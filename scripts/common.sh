#!/bin/bash

OP_VAULT_NAME=${OP_VAULT_NAME:-"Developer Automation"}
OP_ITEM_NAME=${OP_ITEM_NAME:-"EC Dev"}

LOCAL_DEV_DIR=${LOCAL_DEV_ENV_DIR:-./local-dev}
LOCAL_DEV_ENV_FILE=${LOCAL_DEV_ENV_FILE:-${LOCAL_DEV_DIR}/env.sh}

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

    CURRENT_USER=$(id -u -n)

    export CURRENT_USER
}

function ensure_app_channel() {
    ensure_current_user

    if [ -z "${APP_CHANNEL:-}" ]; then
        if [ -z "${CURRENT_USER:-}" ]; then
            fail "CURRENT_USER is not set"
        fi
        APP_CHANNEL="${CURRENT_USER}-dev"
    fi

    local channel_json
    channel_json=$(replicated channel ls -o json | jq -r ".[] | select(.name == \"${APP_CHANNEL}\")")
    if [ -z "$channel_json" ]; then
        replicated channel create --name "${APP_CHANNEL}" --description "EC dev automation" > /dev/null
    fi
    channel_json=$(replicated channel ls -o json | jq -r ".[] | select(.name == \"${APP_CHANNEL}\")")

    if [ -z "$channel_json" ]; then
        fail "Failed to create channel $APP_CHANNEL"
    fi

    APP_CHANNEL=$(echo "$channel_json" | jq -r '.name')
    APP_CHANNEL_ID=$(echo "$channel_json" | jq -r '.id')
    APP_CHANNEL_SLUG=$(echo "$channel_json" | jq -r '.channelSlug')

    export APP_CHANNEL APP_CHANNEL_ID APP_CHANNEL_SLUG
}

function ensure_customer_license_file() {
    if [ -n "${CUSTOMER_LICENSE_FILE:-}" ] && [ -f "${CUSTOMER_LICENSE_FILE}" ]; then
        return
    fi

    export CUSTOMER_LICENSE_FILE
    CUSTOMER_LICENSE_FILE="${LOCAL_DEV_DIR}/${APP_CHANNEL_SLUG}-license.yaml"
    if [ -f "${CUSTOMER_LICENSE_FILE}" ]; then
        return
    fi

    local customer_json
    customer_json=$(replicated customer ls -o json | jq -r ".[] | select(.name == \"${APP_CHANNEL}\")")
    if [ -z "$customer_json" ]; then
        replicated customer create \
            --name "${APP_CHANNEL}" \
            --channel "${APP_CHANNEL_ID}" \
            --type dev \
            --email "${APP_CHANNEL_SLUG}@replicated.com" \
            --airgap --snapshot --kots-install --support-bundle-upload \
            --embedded-cluster-download --embedded-cluster-multinode \
            > /dev/null
    fi

    customer_json=$(replicated customer ls -o json | jq -r ".[] | select(.name == \"${APP_CHANNEL}\")")
    if [ -z "$customer_json" ]; then
        fail "Failed to create customer $APP_CHANNEL"
    fi

    local customer_id
    customer_id=$(echo "$customer_json" | jq -r '.id')

    if ! replicated customer download-license --customer "$customer_id" --output "$CUSTOMER_LICENSE_FILE"; then
        fail "Failed to download license for customer $APP_CHANNEL"
    fi
}

function ensure_local_dev_env() {
    load_local_dev_env

    if [ -n "${APP_CHANNEL_ID:-}" ] && [ -n "${APP_CHANNEL_SLUG:-}" ] && [ -n "${APP_CHANNEL:-}" ]; then
        return
    fi

    ensure_app_channel

    ensure_customer_license_file

    write_local_dev_env_file

    load_local_dev_env

    log "Using app channel $APP_CHANNEL with id $APP_CHANNEL_ID and slug $APP_CHANNEL_SLUG"
}

function write_local_dev_env_file() {
    mkdir -p "$(dirname "${LOCAL_DEV_ENV_FILE}")"

	{
		echo "export APP_CHANNEL=${APP_CHANNEL}"
		echo "export APP_CHANNEL_ID=${APP_CHANNEL_ID}"
		echo "export APP_CHANNEL_SLUG=${APP_CHANNEL_SLUG}"
		echo "export CUSTOMER_LICENSE_FILE=${CUSTOMER_LICENSE_FILE}"
	} > "${LOCAL_DEV_ENV_FILE}"
}

function load_local_dev_env() {
    if [ -f "${LOCAL_DEV_ENV_FILE}" ]; then
		# shellcheck source=/dev/null
        source "${LOCAL_DEV_ENV_FILE}"
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
