#!/bin/bash

set -euo pipefail

START_PAGE=${START_PAGE:-1}
PAGE_SIZE=${PAGE_SIZE:-100}
DRY_RUN=${DRY_RUN:-true}
REPLICATED_API_ORIGIN=${REPLICATED_API_ORIGIN:-"https://api.staging.replicated.com"}
REPLICATED_APP=${REPLICATED_APP:-"embedded-cluster-smoke-test-staging-app"}

# Trim /vendor suffix if present
REPLICATED_API_ORIGIN="${REPLICATED_API_ORIGIN%/vendor}"

# Calculate timestamp for 7 days ago (in seconds since epoch)
if [[ "$OSTYPE" == "darwin"* ]]; then
    # macOS (darwin) syntax
    SEVEN_DAYS_AGO=$(date -v-7d +%s)
else
    # Linux syntax
    SEVEN_DAYS_AGO=$(date -d '7 days ago' +%s)
fi

function get_app_id() {
    local apps_response
    apps_response=$(curl -fsSL -H "Authorization: $REPLICATED_API_TOKEN" "$REPLICATED_API_ORIGIN/vendor/v3/apps")

    local app_id
    app_id=$(echo "$apps_response" | jq -r --arg slug "$REPLICATED_APP" '.apps[] | select(.slug == $slug) | .id')

    if [[ -z "$app_id" || "$app_id" == "null" ]]; then
        echo "Error: Could not find app with slug '$REPLICATED_APP'" >&2
        exit 1
    fi

    echo "$app_id"
}

function demote_old_releases() {
    echo "Fetching channels and releases older than 7 days..."
    if [[ "$OSTYPE" == "darwin"* ]]; then
        echo "Timestamp for 7 days ago: $(date -v-7d)"
    else
        echo "Timestamp for 7 days ago: $(date -d '7 days ago')"
    fi
    echo "---"

    # Get app ID first
    local app_id
    app_id=$(get_app_id)

    # Get all channels
    echo "Fetching channels..."
    local channels_response
    channels_response=$(curl -fsSL -H "Authorization: $REPLICATED_API_TOKEN" "$REPLICATED_API_ORIGIN/vendor/v3/app/$app_id/channels?excludeDetail=true&excludeAdoption=true")

    # Process each channel
    echo "$channels_response" | jq -c '.channels[]' | while read -r channel; do
        local channel_id channel_name
        channel_id=$(echo "$channel" | jq -r '.id')
        channel_name=$(echo "$channel" | jq -r '.name')

        echo "Processing channel: $channel_name (ID: $channel_id)"
        process_channel_releases "$channel_id" "$channel_name" "$app_id"
    done

    echo "Finished processing all channels."
}

function process_channel_releases() {
    local channel_id="$1"
    local channel_name="$2"
    local app_id="$3"
    local current_page=$START_PAGE

    while true; do
        echo "  Processing page $current_page for channel $channel_name..."

        local response

        # Get releases for this channel (excluding already demoted ones)
        response=$(curl -fsSL -H "Authorization: $REPLICATED_API_TOKEN" "$REPLICATED_API_ORIGIN/vendor/v3/app/$app_id/channel/$channel_id/releases?excludeDemoted=true&pageSize=$PAGE_SIZE&currentPage=$current_page")

        # Check if we have releases on this page
        release_count=$(echo "$response" | jq -r '.releases | length')
        if [[ "$release_count" -eq 0 ]]; then
            echo "  No releases found on page $current_page for channel $channel_name, stopping."
            break
        fi

        echo "  Found $release_count releases on page $current_page for channel $channel_name"

        # Loop through each release
        while IFS= read -r release; do
            local created_at timestamp release_time sequence version

            version=$(echo "$release" | jq -r '.semver // "N/A"')
            created_at=$(echo "$release" | jq -r '.created // empty')

            # Skip if createdAt is null
            if [[ -z "$created_at" ]]; then
                echo "  Warning: created is empty for release $version"
                continue
            fi

            # Parse the timestamp and compare
            # Remove milliseconds and ensure proper UTC format
            timestamp=$(echo "$created_at" | sed 's/\.[0-9]*//' | sed 's/Z$//' | sed 's/$/Z/')

            # Validate timestamp format before parsing
            if [[ ! "$timestamp" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$ ]]; then
                echo "  Warning: Invalid timestamp format for release $version: $created_at"
                continue
            fi

            release_time=$(parse_date "$timestamp" +%s)

            # Check if release is older than 7 days
            if [[ "$release_time" -lt "$SEVEN_DAYS_AGO" ]]; then
                # Extract fields from each release
                sequence=$(echo "$release" | jq -r '.channelSequence')

                demote_release "$channel_id" "$channel_name" "$sequence" "$timestamp" "$version" "$app_id"
            fi
        done < <(echo "$response" | jq -c '.releases[]')

        current_page=$((current_page + 1))
    done
}

function demote_release() {
    local channel_id="$1"
    local channel_name="$2"
    local sequence="$3"
    local timestamp="$4"
    local version="$5"
    local app_id="$6"

    local local_time

    # Convert UTC timestamp to local time
    local_time=$(parse_date "$timestamp")

    echo "  Demoting release: Channel: $channel_name, Sequence: $sequence, Created: $local_time, Version: $version"

    if [[ "$DRY_RUN" == "true" ]]; then
        echo "    ✓ DRY RUN"
    else
        local demote_response http_code response_body

        # Demote the release
        demote_response=$(curl -sSL -w "%{http_code}" -X POST -H "Authorization: $REPLICATED_API_TOKEN" -H "Content-Type: application/json" "$REPLICATED_API_ORIGIN/vendor/v3/app/$app_id/channel/$channel_id/release/$sequence/demote" -d '{}')
        http_code="${demote_response: -3}"
        response_body="${demote_response%???}"

        if [[ "$http_code" -eq 200 ]]; then
            echo "    ✓ Successfully demoted release $sequence in channel $channel_name"
        else
            echo "    ✗ Failed to demote release $sequence in channel $channel_name (HTTP $http_code): $response_body"
        fi
    fi
}

# Cross-platform date parsing function
function parse_date() {
    local date_string="$1"
    local format="${2:-}"

    if [[ "$OSTYPE" == "darwin"* ]]; then
        # macOS (darwin) syntax
        if [[ -z "$format" ]]; then
            # Human readable format - treat as UTC and convert to local
            # First convert UTC to epoch, then epoch to local time
            epoch=$(date -j -u -f "%Y-%m-%dT%H:%M:%SZ" "$date_string" +%s 2>/dev/null)
            if [[ -n "$epoch" && "$epoch" != "0" ]]; then
                date -r "$epoch" 2>/dev/null || echo "$date_string"
            else
                echo "$date_string"
            fi
        else
            # Specific format - treat as UTC
            date -j -u -f "%Y-%m-%dT%H:%M:%SZ" "$date_string" "$format" 2>/dev/null || echo "0"
        fi
    else
        # Linux syntax
        if [[ -z "$format" ]]; then
            # Human readable format
            date -d "$date_string" 2>/dev/null || echo "$date_string"
        else
            # Specific format
            date -d "$date_string" "$format" 2>/dev/null || echo "0"
        fi
    fi
}

function main() {
    demote_old_releases
}

main "$@"
