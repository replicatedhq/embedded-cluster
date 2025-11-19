{{- define "troubleshoot.collectors.platform.runPods.analysis-findings" -}}
- runPod:
    name: "collector-analysis-findings"
    collectorName: "collector-analysis-findings"
    namespace: "{{ .Release.Namespace }}"
    timeout: {{ max (.Values.global.pixee.runPodTimeout | regexReplaceAll "[^0-9]" "" | int) 600 }}s
    allowImagePullRetries: true
    podSpec:
      {{- with .Values.global.pixee.utility.image.pullSecrets }}
      imagePullSecrets:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      containers:
        - name: analysis-findings
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

              log_warning() {
                  echo "[WARNING] $1" >&2
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

              # Get findings for an analysis with pagination
              get_analysis_findings() {
                  local analysis_id="$1"
                  local repository_id="$2"
                  local findings_count=0
                  local page_number=0
                  local page_size=50
                  local has_more=true

                  echo ""
                  echo "=========================================="
                  echo "  FINDINGS FOR ANALYSIS: $analysis_id"
                  echo "  REPOSITORY ID: $repository_id"
                  echo "=========================================="

                  while [ "$has_more" = "true" ]; do
                      local findings_url="$API_BASE_URL/api/v1/analyses/$analysis_id/findings?page_number=$page_number&page_size=$page_size"
                      
                      log_info "Fetching findings page $page_number for analysis: $analysis_id"
                      
                      local findings_response
                      if ! findings_response=$(api_request "$findings_url"); then
                          log_warning "Failed to get findings for analysis: $analysis_id"
                          return 1
                      fi

                      # Check if we have items
                      local items_count
                      items_count=$(echo "$findings_response" | jq '._embedded.items | length // 0')

                      if [ "$items_count" -eq 0 ]; then
                          log_info "No more findings found for analysis: $analysis_id"
                          break
                      fi

                      findings_count=$((findings_count + items_count))

                      # Display findings (concise single line format)
                      echo "$findings_response" | jq -r '
                          ._embedded.items[] |
                          "Finding ID: " + (.id // "N/A" | tostring) + " | Rule ID: " + (.rule_id // .rule // "N/A" | tostring)
                      '

                      # Check if there are more pages
                      local total
                      local current_page_size
                      total=$(echo "$findings_response" | jq '.total // 0')
                      current_page_size=$(echo "$findings_response" | jq '.page.size // 0')

                      if [ $(((page_number + 1) * current_page_size)) -ge "$total" ]; then
                          has_more=false
                      else
                          page_number=$((page_number + 1))
                      fi
                  done

                  echo ""
                  echo "Total findings for $analysis_id: $findings_count"
                  echo ""
                  
                  log_success "Completed findings collection for analysis: $analysis_id"
              }

              # Get findings for an analysis using the provided findings URL with pagination
              get_analysis_findings_from_url() {
                  local analysis_id="$1"
                  local repository_name="$2"
                  local base_findings_url="$3"
                  local detector="${4:-unknown}"
                  local findings_count=0
                  local page_number=0
                  local page_size=50
                  local has_more=true

                  echo ""
                  echo "=========================================="
                  echo "  FINDINGS FOR ANALYSIS: $analysis_id"
                  echo "  REPOSITORY ID: $repository_name"
                  echo "  DETECTOR: $detector"
                  echo "=========================================="

                  while [ "$has_more" = "true" ]; do
                      # Add pagination parameters to the provided URL
                      local findings_url="$base_findings_url?page_number=$page_number&page_size=$page_size"
                      
                      log_info "Fetching findings page $page_number for analysis: $analysis_id"
                      
                      local findings_response
                      if ! findings_response=$(api_request "$findings_url"); then
                          log_warning "Failed to get findings for analysis: $analysis_id"
                          return 1
                      fi

                      # Check if we have items
                      local items_count
                      items_count=$(echo "$findings_response" | jq '._embedded.items | length // 0')

                      if [ "$items_count" -eq 0 ]; then
                          log_info "No more findings found for analysis: $analysis_id"
                          break
                      fi

                      findings_count=$((findings_count + items_count))

                      # Display findings (concise single line format)
                      echo "$findings_response" | jq -r '
                          ._embedded.items[] |
                          "Finding ID: " + (.id // "N/A" | tostring) + " | Rule ID: " + (.rule_id // .rule // "N/A" | tostring)
                      '

                      # Check if there are more pages
                      local total
                      local current_page_size
                      total=$(echo "$findings_response" | jq '.total // 0')
                      current_page_size=$(echo "$findings_response" | jq '.page.size // 0')

                      if [ $(((page_number + 1) * current_page_size)) -ge "$total" ]; then
                          has_more=false
                      else
                          page_number=$((page_number + 1))
                      fi
                  done

                  echo ""
                  echo "Total findings for $analysis_id: $findings_count"
                  echo ""
                  
                  log_success "Completed findings collection for analysis: $analysis_id"
              }

              # Process analyses from a scan
              process_scan_analyses() {
                  local scan_id="$1"
                  local repository_name="$2"

                  log_info "Processing analyses for scan: $scan_id"

                  # Get analyses for this scan
                  local analyses_url="$API_BASE_URL/api/v1/scans/$scan_id/analyses"
                  local analyses_response

                  if ! analyses_response=$(api_request "$analyses_url"); then
                      log_warning "Failed to get analyses for scan: $scan_id"
                      return 1
                  fi

                  # Process each analysis
                  echo "$analyses_response" | jq -c '._embedded.items[]' | while read -r analysis; do
                      local analysis_id
                      local analysis_state
                      local analysis_timestamp

                      analysis_id=$(echo "$analysis" | jq -r '.id // "unknown"')
                      analysis_state=$(echo "$analysis" | jq -r '.current_state.state // "unknown"')
                      analysis_timestamp=$(echo "$analysis" | jq -r '.current_state.timestamp // empty')

                      # Only process completed analyses from last 5 days
                      if [ "$analysis_state" = "completed" ] && [ -n "$analysis_timestamp" ] && is_within_last_5_days "$analysis_timestamp"; then
                          log_info "Processing analysis: $analysis_id (state: $analysis_state)"
                          get_analysis_findings "$analysis_id" "$repository_id" || true
                      else
                          log_info "Skipping analysis $analysis_id (state: $analysis_state, timestamp: $analysis_timestamp)"
                      fi
                  done
              }

              # Main function
              main() {
                  log_info "Starting findings extraction from completed analyses"
                  log_info "Processing completed analyses from the last 5 days (since: $FIVE_DAYS_AGO)"

                  echo "=========================================="
                  echo "    PIXEE FINDINGS EXTRACTION"
                  echo "=========================================="
                  echo "Collection Time: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
                  echo "API Base URL: $API_BASE_URL"
                  echo "Collecting from completed analyses since: $FIVE_DAYS_AGO"
                  echo ""

                  # Detect SSL requirements once at startup
                  CURL_INSECURE_FLAG=$(detect_ssl_requirements)
                  export CURL_INSECURE_FLAG

                  # Get completed analyses directly
                  log_info "Fetching completed analyses from: $API_BASE_URL/api/v1/analyses?page_number=0&page_size=10&states=completed"
                  local analyses_response
                  if ! analyses_response=$(api_request "$API_BASE_URL/api/v1/analyses?page_number=0&page_size=10&states=completed"); then
                      log_error "Failed to fetch analyses - this may be expected if the platform service is not accessible"
                      exit 0  # Exit gracefully as this is a support bundle collection
                  fi

                  log_success "Successfully fetched completed analyses"

                  # Parse analyses and extract findings
                  local analysis_count
                  analysis_count=$(echo "$analyses_response" | jq '._embedded.items | length // 0')

                  if [ "$analysis_count" -eq 0 ]; then
                      log_warning "No completed analyses found"
                      exit 0
                  fi

                  log_info "Found $analysis_count completed analyses to process"

                  echo "$analyses_response" | jq -c '._embedded.items[]' | while read -r analysis; do
                      local analysis_id
                      local analysis_state
                      local analysis_timestamp
                      local findings_url
                      local repo_id
                      local detector

                      analysis_id=$(echo "$analysis" | jq -r '.id // "unknown"')
                      analysis_state=$(echo "$analysis" | jq -r '.current_state.state // "unknown"')
                      analysis_timestamp=$(echo "$analysis" | jq -r '.current_state.timestamp // empty')
                      findings_url=$(echo "$analysis" | jq -r '._links.findings.href // empty')
                      
                      # Extract repository ID and detector from embedded scan data if available
                      repo_id=$(echo "$analysis" | jq -r '._embedded.scan._links.repository.href // "unknown"' | sed 's|.*/||')
                      detector=$(echo "$analysis" | jq -r '._embedded.scan.detector // "unknown"')

                      echo ""
                      echo "=========================================="
                      echo "PROCESSING ANALYSIS: $analysis_id"
                      echo "REPOSITORY ID: $repo_id"
                      echo "DETECTOR: $detector"
                      echo "STATE: $analysis_state"
                      echo "TIMESTAMP: $analysis_timestamp"
                      echo "=========================================="

                      # Process only completed analyses from last 5 days
                      if [ "$analysis_state" = "completed" ] && [ -n "$analysis_timestamp" ] && is_within_last_5_days "$analysis_timestamp"; then
                          log_info "Processing analysis: $analysis_id (state: $analysis_state, timestamp: $analysis_timestamp)"
                          
                          if [ -n "$findings_url" ] && [ "$findings_url" != "null" ]; then
                              # Convert relative URL to absolute URL if needed
                              if echo "$findings_url" | grep -q "^/"; then
                                  findings_url="$API_BASE_URL$findings_url"
                              fi
                              
                              # Get findings directly from the analysis
                              get_analysis_findings_from_url "$analysis_id" "$repo_id" "$findings_url" "$detector" || true
                          else
                              log_warning "No findings URL found for analysis: $analysis_id"
                          fi
                      else
                          log_info "Skipping analysis $analysis_id (state: $analysis_state, timestamp: $analysis_timestamp) - not completed or outside 5-day window"
                      fi
                  done

                  echo ""
                  echo "=========================================="
                  echo "    FINDINGS EXTRACTION COMPLETE"
                  echo "=========================================="
                  
                  log_success "Findings extraction process completed successfully"
              }

              # Run main function
              main
{{- end -}}