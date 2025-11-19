{{- define "troubleshoot.collectors.platform.runPods.analysis-status" -}}
- runPod:
    name: "collector-analysis-status"
    collectorName: "collector-analysis-status"
    namespace: "{{ .Release.Namespace }}"
    timeout:  {{ max (.Values.global.pixee.runPodTimeout | regexReplaceAll "[^0-9]" "" | int) 600 }}s
    allowImagePullRetries: true
    podSpec:
      {{- with .Values.global.pixee.utility.image.pullSecrets }}
      imagePullSecrets:
        {{- toYaml . | nindent 8 }}
      {{- end }}

      containers:
        - name: analysis-status
          image: {{ printf "%s/%s:%s" .Values.global.pixee.utility.image.registry .Values.global.pixee.utility.image.repository .Values.global.pixee.utility.image.tag | quote }}
          env:
            - name: API_BASE_URL
              value: "{{ .Values.global.pixee.protocol }}://{{ .Values.global.pixee.domain }}"
            {{- if and .Values.global.pixee.authentication.enabled (or .Values.global.pixee.authentication.restApi.existingSecret .Values.global.pixee.authentication.restApi.apiKey) }}
            - name: API_KEY
              {{- if .Values.global.pixee.authentication.restApi.existingSecret }}
              valueFrom:
                secretKeyRef:
                  name: {{ .Values.global.pixee.authentication.restApi.existingSecret }}
                  key: {{ .Values.global.pixee.authentication.restApi.secretKeys.apiKey }}
              {{- else }}
              value: "{{ .Values.global.pixee.authentication.restApi.apiKey }}"
              {{- end }}
              {{- end }}
          command: ["/bin/sh"]
          args: 
            - -c
            - |
              #!/bin/sh
              set -eu

              # Calculate timestamp for 5 days ago (Alpine Linux compatible)
              current_epoch=$(date -u +%s)
              five_days_seconds=$((5 * 24 * 60 * 60))
              cutoff_epoch=$((current_epoch - five_days_seconds))
              FIVE_DAYS_AGO=$(date -u -d "@$cutoff_epoch" +%Y-%m-%dT00:00:00Z 2>/dev/null || date -u +%Y-%m-%dT00:00:00Z -d "@$cutoff_epoch" 2>/dev/null || {
                  # Simple calculation for basic date command
                  cutoff_date=$(date -u +%Y-%m-%d -d "@$cutoff_epoch" 2>/dev/null || date -u +%Y-%m-%d)
                  echo "${cutoff_date}T00:00:00Z"
              })

              # Logging functions
              log_info() {
                  echo "[INFO] $1" >&2
              }

              log_error() {
                  echo "[ERROR] $1" >&2
              }

              log_success() {
                  echo "[SUCCESS] $1" >&2
              }

              # Detect if SSL certificate requires --insecure flag
              detect_ssl_requirements() {
                  log_info "Testing SSL certificate for $API_BASE_URL"
                  
                  # Test if regular curl works
                  if curl -s --connect-timeout 5 --max-time 10 "$API_BASE_URL" > /dev/null 2>&1; then
                      log_info "SSL certificate is valid, using secure connection"
                      echo ""
                  else
                      # If regular curl fails, test with --insecure
                      if curl -s --insecure --connect-timeout 5 --max-time 10 "$API_BASE_URL" > /dev/null 2>&1; then
                          log_info "SSL certificate requires --insecure flag (likely self-signed)"
                          echo "--insecure"
                      else
                          log_info "Connection failed even with --insecure, will try without SSL verification"
                          echo "--insecure"
                      fi
                  fi
              }

              # Make API request with error handling
              api_request() {
                  local url="$1"
                  local temp_file
                  temp_file=$(mktemp)
                  local http_code

                  log_info "Making API request to: $url"

                  # Build curl command with conditional Authorization header
                  if [ -n "${API_KEY:-}" ]; then
                      http_code=$(curl -s -o "$temp_file" -w '%{http_code}' $CURL_INSECURE_FLAG \
                          -H 'Accept: application/json' \
                          -H "Authorization: Bearer $API_KEY" \
                          "$url")
                  else
                      http_code=$(curl -s -o "$temp_file" -w '%{http_code}' $CURL_INSECURE_FLAG \
                          -H 'Accept: application/json' \
                          "$url")
                  fi

                  if [ "$http_code" -ge 200 ] && [ "$http_code" -lt 300 ]; then
                      cat "$temp_file"
                      rm -f "$temp_file"
                  else
                      log_error "API request failed with HTTP $http_code for URL: $url"
                      cat "$temp_file" >&2
                      rm -f "$temp_file"
                      return 1
                  fi
              }

              # Check if timestamp is within last 5 days (Alpine Linux compatible)
              is_within_last_5_days() {
                  local timestamp="$1"
                  local timestamp_epoch
                  local five_days_ago_epoch=$cutoff_epoch

                  # Convert timestamp to epoch (Alpine Linux compatible)
                  # Handle fractional seconds by removing them
                  local clean_timestamp=$(echo "$timestamp" | sed 's/\.[0-9]*Z/Z/')
                  
                  # Parse timestamp manually for Alpine compatibility
                  local year month day hour minute second
                  year=$(echo "$clean_timestamp" | cut -d'T' -f1 | cut -d'-' -f1)
                  month=$(echo "$clean_timestamp" | cut -d'T' -f1 | cut -d'-' -f2)
                  day=$(echo "$clean_timestamp" | cut -d'T' -f1 | cut -d'-' -f3)
                  hour=$(echo "$clean_timestamp" | cut -d'T' -f2 | cut -d':' -f1)
                  minute=$(echo "$clean_timestamp" | cut -d'T' -f2 | cut -d':' -f2)
                  second=$(echo "$clean_timestamp" | cut -d'T' -f2 | cut -d':' -f3 | sed 's/Z//')
                  
                  # Convert to epoch using date command with individual components
                  timestamp_epoch=$(date -u -d "$year-$month-$day $hour:$minute:$second" +%s 2>/dev/null || {
                      # Fallback: simple comparison using current epoch and rough calculation
                      current_epoch=$(date -u +%s)
                      # If timestamp looks recent (within reasonable bounds), assume it's valid
                      echo "$current_epoch"
                  })

                  log_info "Comparing timestamp_epoch: $timestamp_epoch vs five_days_ago_epoch: $five_days_ago_epoch"
                  
                  if [ "$timestamp_epoch" -gt "$five_days_ago_epoch" ]; then
                      return 0
                  else
                      return 1
                  fi
              }

              # Function to filter analyses by timestamp (last 5 days)
              filter_recent_analyses() {
                  local json_data="$1"
                  local filtered_items="[]"
                  
                  echo "$json_data" | jq -c '.items[]' | while read -r item; do
                      local timestamp=$(echo "$item" | jq -r '.current_state.timestamp // empty')
                      
                      if [ -n "$timestamp" ] && is_within_last_5_days "$timestamp"; then
                          echo "$item"
                      fi
                  done | jq -s '{items: ., total: length}'
              }

              # Function to display analyses for a given state
              display_analyses() {
                  local state="$1"
                  local json_data="$2"

                  local count=$(echo "$json_data" | jq -r '.total // 0')

                  echo ""
                  echo "=========================================="
                  echo "  $state ANALYSES ($count found)"
                  echo "=========================================="

                  if [ "$count" -eq 0 ]; then
                      echo "No $state analyses found in the last 5 days."
                      echo ""
                      return
                  fi

                  echo "$json_data" | jq -r '
                      .items[] |
                      "Analysis ID: " + .id +
                      "\nRepository ID: " + (._embedded.scan._links.repository.href // "Unknown" | split("/") | last) +
                      "\nSHA: " + .sha +
                      "\nBranch: " + (.branch // "N/A") +
                      "\nCurrent State: " + .current_state.state +
                      "\nTimestamp: " + .current_state.timestamp +
                      "\nDetector: " + ._embedded.scan.detector +
                      "\n" + ("-" * 60)
                  '
                  echo ""
              }

              # Collect analyses for a specific state (display version)
              collect_and_display_analyses() {
                  local state_filter="$1"
                  local state_label="$2"

                  log_info "Fetching $state_label analyses..."

                  local url="$API_BASE_URL/api/v1/analyses?page_number=0&page_size=100&$state_filter"
                  local response

                  if ! response=$(api_request "$url"); then
                      log_error "Failed to fetch $state_label analyses"
                      return 1
                  fi

                  # Format data and filter for last 5 days
                  local formatted_data=$(echo "$response" | jq '{
                      items: (._embedded.items // []),
                      total: (._embedded.items // [] | length)
                  }')

                  local recent_data=$(filter_recent_analyses "$formatted_data")

                  # Display the results
                  display_analyses "$state_label" "$recent_data"

                  log_success "Completed collection of $state_label analyses"
              }

              # Collect analyses for a specific state (data only)
              collect_analyses_data() {
                  local state_filter="$1"

                  local url="$API_BASE_URL/api/v1/analyses?page_number=0&page_size=100&$state_filter"
                  local response

                  if ! response=$(api_request "$url" 2>/dev/null); then
                      echo '{"items": [], "total": 0}'
                      return 1
                  fi

                  # Format data and filter for last 5 days
                  local formatted_data=$(echo "$response" | jq '{
                      items: (._embedded.items // []),
                      total: (._embedded.items // [] | length)
                  }')

                  filter_recent_analyses "$formatted_data"
              }

              # Main function
              main() {
                  log_info "Starting analyses collection process"
                  log_info "Collecting analyses from the last 5 days (since: $FIVE_DAYS_AGO)"

                  echo "=========================================="
                  echo "    PIXEE ANALYSES SUPPORT BUNDLE"
                  echo "=========================================="
                  echo "Collection Time: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
                  echo "API Base URL: $API_BASE_URL"
                  echo "Collecting data since: $FIVE_DAYS_AGO"
                  echo ""

                  # Detect SSL requirements once at startup
                  CURL_INSECURE_FLAG=$(detect_ssl_requirements)
                  export CURL_INSECURE_FLAG

                  # Display analyses
                  collect_and_display_analyses "states=queued" "QUEUED"
                  collect_and_display_analyses "states=in_progress" "IN-PROGRESS"
                  collect_and_display_analyses "states=completed" "COMPLETED"

                  # Collect data for summary (separate calls to avoid output mixing)
                  local queued_data=$(collect_analyses_data "states=queued")
                  local in_progress_data=$(collect_analyses_data "states=in_progress")
                  local completed_data=$(collect_analyses_data "states=completed")

                  # Generate summary
                  local total_queued=$(echo "$queued_data" | jq -r '.total')
                  local total_in_progress=$(echo "$in_progress_data" | jq -r '.total')
                  local total_completed=$(echo "$completed_data" | jq -r '.total')
                  local total_all=$((total_queued + total_in_progress + total_completed))

                  echo ""
                  echo "=========================================="
                  echo "    SUMMARY"
                  echo "=========================================="
                  echo "Queued: $total_queued"
                  echo "In Progress: $total_in_progress"
                  echo "Completed: $total_completed"
                  echo "Total (last 5 days): $total_all"

                  # Repository breakdown
                  echo ""
                  echo "=========================================="
                  echo "    BY REPOSITORIES"
                  echo "=========================================="

                  local all_data=$(echo "$queued_data $in_progress_data $completed_data" | jq -s '
                      [.[] | .items[]] |
                      group_by(._embedded.scan._links.repository.href // "Unknown" | split("/") | last) |
                      map({
                          repository_id: (.[0]._embedded.scan._links.repository.href // "Unknown" | split("/") | last),
                          count: length,
                          states: [.[] | .current_state.state] | group_by(.) | map({state: .[0], count: length}),
                          detectors: [.[] | ._embedded.scan.detector // "Unknown"] | unique
                      }) |
                      sort_by(.repository_id)
                  ')

                  echo "$all_data" | jq -r '
                      .[] |
                      "Repository ID: " + .repository_id +
                      "\n  Total analyses: " + (.count | tostring) +
                      "\n  Detectors: " + (.detectors | join(", ")) +
                      "\n  States: " + ([.states[] | "  " + .state + ": " + (.count | tostring)] | join("\n         ")) +
                      "\n"
                  '

                  echo ""
                  echo "=========================================="
                  echo "    COLLECTION COMPLETE"
                  echo "=========================================="

                  log_success "Analyses collection process completed successfully"
              }

              # Run main function
              main
{{- end -}}