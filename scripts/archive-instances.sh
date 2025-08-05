#!/bin/bash

set -euo pipefail

COUNT=${COUNT:-500}
DRY_RUN=${DRY_RUN:-true}
REPLICATED_API_ORIGIN=${REPLICATED_API_ORIGIN:-"https://api.staging.replicated.com"}
REPLICATED_APP=${REPLICATED_APP:-"embedded-cluster-smoke-test-staging-app"}

# Trim /vendor suffix if present
REPLICATED_API_ORIGIN="${REPLICATED_API_ORIGIN%/vendor}"

# Calculate timestamp for 1 hour ago (in seconds since epoch)
if [[ "$OSTYPE" == "darwin"* ]]; then
    # macOS (darwin) syntax
    ONE_HOUR_AGO=$(date -v-1H +%s)
else
    # Linux syntax
    ONE_HOUR_AGO=$(date -d '1 hour ago' +%s)
fi

function archive_instances() {
    echo "Fetching instances with lastCheckinAt greater than 1 hour ago..."
    if [[ "$OSTYPE" == "darwin"* ]]; then
        echo "Timestamp for 1 hour ago: $(date -v-1H)"
    else
        echo "Timestamp for 1 hour ago: $(date -d '1 hour ago')"
    fi
    echo "---"

    # Get the JSON response and extract instances
    RESPONSE=$(curl -fsSL -H "Authorization: $REPLICATED_API_TOKEN" "$REPLICATED_API_ORIGIN/v1/instances?appSelector=$REPLICATED_APP&pageSize=$COUNT&currentPage=1")

    # Loop through each instance using jq to extract the array
    echo "$RESPONSE" | jq -c '.instances[]' | while read -r instance; do
        # Extract fields from each instance
        ID=$(echo "$instance" | jq -r '.id')
        LAST_CHECKIN=$(echo "$instance" | jq -r '.lastCheckinAt // empty')
        VERSION=$(echo "$instance" | jq -r '.versionLabel // "N/A"')
        CHANNEL=$(echo "$instance" | jq -r '._embedded.channel.name // "N/A"')

        # Skip if lastCheckinAt is null
        if [[ -z "$LAST_CHECKIN" ]]; then
            continue
        fi

        maybe_archive_instance "$ID" "$LAST_CHECKIN" "$VERSION" "$CHANNEL"
    done
}

function maybe_archive_instance() {
    local ID="$1"
    local LAST_CHECKIN="$2"
    local VERSION="$3"
    local CHANNEL="$4"

    # Parse the timestamp and compare
    # Remove milliseconds and ensure proper UTC format
    TIMESTAMP=$(echo "$LAST_CHECKIN" | sed 's/\.[0-9]*//' | sed 's/Z$//' | sed 's/$/Z/')

    # Validate timestamp format before parsing
    if [[ ! "$TIMESTAMP" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$ ]]; then
        echo "Warning: Invalid timestamp format for instance $ID: $LAST_CHECKIN"
        return
    fi

    INSTANCE_TIME=$(parse_date "$TIMESTAMP" +%s)
    # Convert UTC timestamp to local time
    LOCAL_TIME=$(parse_date "$TIMESTAMP")

    # Check if instance hasn't checked in for more than 1 hour
    if [[ "$INSTANCE_TIME" -lt "$ONE_HOUR_AGO" ]]; then
        echo "Archiving instance: ID: $ID, LastCheckin: $LOCAL_TIME, Version: $VERSION, Channel: $CHANNEL"

        if [[ "$DRY_RUN" == "true" ]]; then
            echo "  ✓ DRY RUN"
        else
            # Archive the instance
            ARCHIVE_RESPONSE=$(curl -sSL -w "%{http_code}" -X POST -H "Authorization: $REPLICATED_API_TOKEN" -H "Content-Type: application/json" "$REPLICATED_API_ORIGIN/vendor/v3/instance/$ID/archive" -d '{}')
            HTTP_CODE="${ARCHIVE_RESPONSE: -3}"
            RESPONSE_BODY="${ARCHIVE_RESPONSE%???}"

            if [[ "$HTTP_CODE" -eq 200 ]]; then
                echo "  ✓ Successfully archived instance $ID"
            else
                echo "  ✗ Failed to archive instance $ID (HTTP $HTTP_CODE): $RESPONSE_BODY"
            fi
        fi
    else
        echo "Instance $ID has checked in within the last hour ($LOCAL_TIME), skipping..."
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
    archive_instances
}

main "$@"
