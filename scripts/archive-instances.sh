#!/bin/bash

set -euo pipefail

COUNT=${COUNT:-200}
DRY_RUN=${DRY_RUN:-true}
REPLICATED_API_ORIGIN=${REPLICATED_API_ORIGIN:-"https://api.staging.replicated.com"}
REPLICATED_APP=${REPLICATED_APP:-"embedded-cluster-smoke-test-staging-app"}

# Trim /vendor suffix if present
REPLICATED_API_ORIGIN="${REPLICATED_API_ORIGIN%/vendor}"

function archive_instances() {
    # Calculate timestamp for 1 hour ago (in seconds since epoch) - macOS compatible
    ONE_HOUR_AGO=$(date -v-1H +%s)

    echo "Fetching instances with lastCheckinAt greater than 1 hour ago..."
    echo "Timestamp for 1 hour ago: $(date -v-1H)"
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
    TIMESTAMP="${LAST_CHECKIN%%.*}Z"
    INSTANCE_TIME=$(date -j -f "%Y-%m-%dT%H:%M:%SZ" "$TIMESTAMP" +%s 2>/dev/null || echo "0")
    
    # Check if instance hasn't checked in for more than 1 hour
    if [[ "$INSTANCE_TIME" -lt "$ONE_HOUR_AGO" ]]; then
        # Convert UTC timestamp to local time
        LOCAL_TIME=$(date -j -f "%Y-%m-%dT%H:%M:%SZ" "$TIMESTAMP" 2>/dev/null || echo "$LAST_CHECKIN")
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
    fi
}

function main() {
    archive_instances
}

main "$@"
