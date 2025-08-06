#!/bin/bash

set -euo pipefail

START_PAGE=${START_PAGE:-1}
PAGE_SIZE=${PAGE_SIZE:-100}
DRY_RUN=${DRY_RUN:-true}
REPLICATED_API_ORIGIN=${REPLICATED_API_ORIGIN:-"https://api.staging.replicated.com"}
REPLICATED_APP=${REPLICATED_APP:-"embedded-cluster-smoke-test-staging-app"}

# Trim /vendor suffix if present
REPLICATED_API_ORIGIN="${REPLICATED_API_ORIGIN%/vendor}"

# Calculate timestamp for 1 day ago (in seconds since epoch)
if [[ "$OSTYPE" == "darwin"* ]]; then
    # macOS (darwin) syntax
    ONE_DAY_AGO=$(date -v-1d +%s)
else
    # Linux syntax
    ONE_DAY_AGO=$(date -d '1 day ago' +%s)
fi

function archive_instances() {
    echo "Fetching instances with lastCheckinAt greater than 1 day ago..."
    if [[ "$OSTYPE" == "darwin"* ]]; then
        echo "Timestamp for 1 day ago: $(date -v-1d)"
    else
        echo "Timestamp for 1 day ago: $(date -d '1 day ago')"
    fi
    echo "---"

    local current_page=$START_PAGE

    while true; do
        echo "Processing page $current_page..."

        local response

        # Get the JSON response and extract instances for current page
        response=$(curl -fsSL -H "Authorization: $REPLICATED_API_TOKEN" "$REPLICATED_API_ORIGIN/v1/instances?appSelector=$REPLICATED_APP&pageSize=$PAGE_SIZE&currentPage=$current_page")

        # Check if we have instances on this page
        instance_count=$(echo "$response" | jq -r '.instances | length')
        if [[ "$instance_count" -eq 0 ]]; then
            echo "No instances found on page $current_page, stopping."
            break
        fi

        echo "Found $instance_count instances on page $current_page"

        # Loop through each instance using jq to extract the array
        echo "$response" | jq -c '.instances[]' | while read -r instance; do
            local id last_checkin timestamp instance_time id version channel

            id=$(echo "$instance" | jq -r '.id')
            last_checkin=$(echo "$instance" | jq -r '.lastCheckinAt // empty')

            # Skip if lastCheckinAt is null
            if [[ -z "$last_checkin" ]]; then
                echo "Warning: lastCheckinAt is empty for instance $id"
                continue
            fi

            # Parse the timestamp and compare
            # Remove milliseconds and ensure proper UTC format
            timestamp=$(echo "$last_checkin" | sed 's/\.[0-9]*//' | sed 's/Z$//' | sed 's/$/Z/')

            # Validate timestamp format before parsing
            if [[ ! "$timestamp" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$ ]]; then
                echo "Warning: Invalid timestamp format for instance $id: $last_checkin"
                continue
            fi

            instance_time=$(parse_date "$timestamp" +%s)

            # Skip if parsing failed (returns -1)
            if [[ "$instance_time" -eq -1 ]]; then
                echo "Warning: Failed to parse timestamp $timestamp for instance $id: $last_checkin"
                continue
            fi

            # Check if instance hasn't checked in for more than 1 day
            if [[ "$instance_time" -lt "$ONE_DAY_AGO" ]]; then
                # Extract fields from each instance
                version=$(echo "$instance" | jq -r '.versionLabel // "N/A"')
                channel=$(echo "$instance" | jq -r '._embedded.channel.name // "N/A"')

                archive_instance "$id" "$timestamp" "$version" "$channel"
            fi
        done

        current_page=$((current_page + 1))
    done

    echo "Finished processing all pages."
}

function archive_instance() {
    local id="$1"
    local timestamp="$2"
    local version="$3"
    local channel="$4"

    local local_time

    # Convert UTC timestamp to local time
    local_time=$(parse_date "$timestamp")

    echo "Archiving instance: ID: $id, LastCheckin: $local_time, Version: $version, Channel: $channel"

    if [[ "$DRY_RUN" == "true" ]]; then
        echo "  ✓ DRY RUN"
    else
        local archive_response http_code response_body

        # Archive the instance
        archive_response=$(curl -sSL -w "%{http_code}" -X POST -H "Authorization: $REPLICATED_API_TOKEN" -H "Content-Type: application/json" "$REPLICATED_API_ORIGIN/vendor/v3/instance/$id/archive" -d '{}')
        http_code="${archive_response: -3}"
        response_body="${archive_response%???}"

        if [[ "$http_code" -eq 200 ]]; then
            echo "  ✓ Successfully archived instance $id"
        else
            echo "  ✗ Failed to archive instance $id (HTTP $http_code): $response_body"
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
            date -j -u -f "%Y-%m-%dT%H:%M:%SZ" "$date_string" "$format" 2>/dev/null || echo "-1"
        fi
    else
        # Linux syntax
        if [[ -z "$format" ]]; then
            # Human readable format
            date -d "$date_string" 2>/dev/null || echo "$date_string"
        else
            # Specific format
            date -d "$date_string" "$format" 2>/dev/null || echo "-1"
        fi
    fi
}

function main() {
    archive_instances
}

main "$@"
