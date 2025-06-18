# Infrastructure Setup API with Preflight Validation

## Overview

This document describes the API changes implemented to enforce preflight validation during infrastructure setup. The changes ensure that infrastructure setup can only proceed when host preflights pass OR when they fail but the operator has explicitly allowed bypassing them via the `--ignore-host-preflights` CLI flag.

## Architecture

### High-Level Flow

1. **Frontend**: Sends infrastructure setup request with user intent
2. **Backend**: Validates current preflight status against CLI configuration  
3. **Backend**: Only proceeds if preflights pass OR (preflights fail + CLI flag allows + user confirms)
4. **Response**: Includes context about whether preflights were ignored

### Security Model

- **CLI Flag Enforcement**: Only servers started with `--ignore-host-preflights` can bypass preflight failures
- **User Confirmation Required**: Even with CLI flag, user must explicitly confirm in UI
- **Audit Trail**: API responses include whether preflights were bypassed for transparency

## API Changes

### 1. New Request Type: `InfraSetupRequest`

**File**: `api/types/infra.go`

```go
// InfraSetupRequest represents a request to set up infrastructure
type InfraSetupRequest struct {
    IgnorePreflightFailures bool `json:"ignorePreflightFailures"`
}
```

**Purpose**: Allows frontend to communicate user intent about handling preflight failures.

**Values**:
- `true`: User confirmed they want to proceed despite preflight failures
- `false`: User expects preflights to pass before proceeding

### 2. New Response Type: `InfraSetupResponse`

**File**: `api/types/infra.go`

```go
// InfraSetupResponse represents the response from setting up infrastructure
type InfraSetupResponse struct {
    *Infra                  `json:",inline"`
    PreflightsIgnored       bool `json:"preflightsIgnored"`
}
```

**Purpose**: Provides transparency about installation context for debugging and audit purposes.

**Fields**:
- **`*Infra`**: Standard infrastructure status (components, logs, status)
- **`PreflightsIgnored`**: Boolean indicating whether preflight failures were bypassed

### 3. Updated API Endpoint

**Endpoint**: `POST /api/install/infra/setup`

**Before**:
```http
POST /api/install/infra/setup
Authorization: Bearer <token>

(no body)
```

**After**:
```http
POST /api/install/infra/setup
Authorization: Bearer <token>
Content-Type: application/json

{
  "ignorePreflightFailures": false
}
```

**Response Changes**:
```json
{
  "components": [...],
  "logs": "...",
  "status": {...},
  "preflightsIgnored": false
}
```

## Validation Logic

### New Validation Function: `ValidatePreflightStatus`

**File**: `api/controllers/install/preflight_validation.go`

```go
func ValidatePreflightStatus(preflightStatus *types.HostPreflights, ignorePreflightFailures bool) error
```

**Validation Rules**:

1. **Preflights Pass**: ✅ Always allowed
   ```
   Preflight Status: Success → Allow Setup
   ```

2. **Preflights Fail + No CLI Flag**: ❌ Blocked
   ```
   Preflight Status: Failed
   CLI Flag: --ignore-host-preflights NOT used
   → Error: "Preflight checks failed"
   ```

3. **Preflights Fail + CLI Flag + User Confirms**: ✅ Allowed
   ```
   Preflight Status: Failed
   CLI Flag: --ignore-host-preflights used
   User Intent: ignorePreflightFailures = true
   → Allow Setup (with audit trail)
   ```

4. **Preflights Fail + CLI Flag + User Doesn't Confirm**: ❌ Blocked
   ```
   Preflight Status: Failed
   CLI Flag: --ignore-host-preflights used
   User Intent: ignorePreflightFailures = false
   → Error: "Preflight checks failed"
   ```

5. **Preflights Fail + User Confirms + No CLI Flag**: ❌ Blocked
   ```
   Preflight Status: Failed
   CLI Flag: --ignore-host-preflights NOT used
   User Intent: ignorePreflightFailures = true
   → Error: "Cannot ignore preflight failures without --ignore-host-preflights flag"
   ```

### Handler Integration

**File**: `api/install.go` - `postInstallSetupInfra`

The handler now:
1. Parses the request body to get user intent
2. Retrieves current preflight status
3. Validates using `ValidatePreflightStatus`
4. Returns appropriate HTTP status codes:
   - `400 Bad Request`: Validation failures
   - `200 OK`: Successful setup with context

## Error Handling

### Error Responses

**Preflight Validation Failure**:
```json
HTTP 400 Bad Request
{
  "statusCode": 400,
  "message": "Preflight checks failed. Cannot proceed with installation."
}
```

**CLI Flag Not Set**:
```json
HTTP 400 Bad Request
{
  "statusCode": 400,
  "message": "Cannot ignore preflight failures without --ignore-host-preflights flag"
}
```

**Network/System Errors**:
```json
HTTP 500 Internal Server Error
{
  "statusCode": 500,
  "message": "Failed to validate preflight status: <details>"
}
```

## Frontend Integration

### UI Behavior Changes

The frontend now:

1. **Receives CLI flag status** via `allowIgnoreHostPreflights` in preflight responses
2. **Enables/disables button** based on preflight status and CLI flag
3. **Shows confirmation modal** when preflights fail but CLI flag allows override
4. **Sends explicit intent** via `ignorePreflightFailures` parameter

### Request Examples

**Normal Installation (preflights pass)**:
```javascript
fetch('/api/install/infra/setup', {
  method: 'POST',
  headers: {
    'Content-Type': 'application/json',
    'Authorization': 'Bearer <token>'
  },
  body: JSON.stringify({
    ignorePreflightFailures: false
  })
})
```

**Override Installation (user confirmed)**:
```javascript
fetch('/api/install/infra/setup', {
  method: 'POST',
  headers: {
    'Content-Type': 'application/json',
    'Authorization': 'Bearer <token>'
  },
  body: JSON.stringify({
    ignorePreflightFailures: true
  })
})
```

## Migration Considerations

### Backward Compatibility

- **API Version**: No version change required
- **Request Format**: New request body is optional for backward compatibility
- **Response Format**: New field added without breaking existing fields

### Deployment

1. **Backend Deploy**: New validation logic will immediately enforce preflight checks
2. **Frontend Deploy**: Enhanced UI provides better user experience
3. **CLI Flag**: Operators must use `--ignore-host-preflights` to override failures

## Security Considerations

### Threat Model

1. **Unauthorized Bypass**: Prevented by CLI flag requirement
2. **Accidental Override**: Prevented by explicit user confirmation requirement
3. **Audit Trail**: `preflightsIgnored` field provides permanent record

### Best Practices

- CLI flag should only be used in emergency situations
- Operators should understand the risks of bypassing preflight checks
- Monitor `preflightsIgnored: true` installations for issues

## Examples

### Successful Installation Flow

```bash
# 1. Start server with CLI flag
./embedded-cluster install --ignore-host-preflights

# 2. Preflights fail, user confirms override in UI
# 3. API request sent:
POST /api/install/infra/setup
{
  "ignorePreflightFailures": true
}

# 4. API response:
{
  "components": [...],
  "status": {"state": "Running"},
  "preflightsIgnored": true
}
```

### Blocked Installation Flow

```bash
# 1. Start server without CLI flag
./embedded-cluster install

# 2. Preflights fail, API request sent:
POST /api/install/infra/setup
{
  "ignorePreflightFailures": false
}

# 3. API response:
HTTP 400 Bad Request
{
  "statusCode": 400,
  "message": "Preflight checks failed. Cannot proceed with installation."
}
```

## Related Files

### Backend Files
- `api/types/infra.go` - Request/response types
- `api/install.go` - API handler
- `api/controllers/install/preflight_validation.go` - Validation logic
- `api/integration/infra_test.go` - Integration tests

### Frontend Files  
- `web/src/components/wizard/ValidationStep.tsx` - UI component
- `web/src/components/wizard/tests/ValidationStep.test.tsx` - Component tests

### Configuration
- `cmd/installer/cli/install.go` - CLI flag definition
- `cmd/installer/cli/api.go` - CLI flag wiring

---

*This document describes the implementation completed as part of the infrastructure setup preflight validation feature.* 