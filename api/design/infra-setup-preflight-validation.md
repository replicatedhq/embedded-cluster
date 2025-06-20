# Infrastructure Setup API with Preflight Validation

## Overview

The infrastructure setup API enforces preflight validation to ensure system readiness before proceeding with installation. The system requires that host preflights either pass successfully OR that they fail but the operator has explicitly enabled bypass capabilities and the user confirms the override.

## Design Principles

### Security-First Approach
- **Explicit Configuration**: Bypass capability must be explicitly enabled via CLI flag
- **User Confirmation**: Even with bypass enabled, user must explicitly confirm the override
- **Fail-Safe Default**: Without explicit configuration, failed preflights block installation

### API Consistency
- Both setup and status endpoints return the same `Infra` response type
- Consistent error handling and HTTP status codes
- Predictable API behavior regardless of preflight state

## Architecture

### Request Flow

```
Frontend Request → API Handler → SetupInfra Controller → Preflight Validation → Infrastructure Setup
```

1. **Frontend** sends setup request with user intent about handling preflight failures
2. **API Handler** parses request and delegates to controller
3. **SetupInfra Controller** validates preflight status against configuration
4. **Infrastructure Setup** proceeds only if validation passes

### Validation Logic

The system implements a multi-factor validation approach:

| Preflight Status | CLI Flag Enabled | User Confirms Override | Result |
|------------------|------------------|------------------------|---------|
| ✅ Pass | Any | Any | ✅ Proceed |
| ❌ Fail | ❌ No | Any | ❌ Block |
| ❌ Fail | ✅ Yes | ❌ No | ❌ Block |
| ❌ Fail | ✅ Yes | ✅ Yes | ✅ Proceed |

## API Specification

### Setup Infrastructure

**Endpoint**: `POST /api/install/infra/setup`

**Request**:
```json
{
  "ignorePreflightFailures": boolean
}
```

**Parameters**:
- `ignorePreflightFailures`: User intent to proceed despite preflight failures

**Response** (Success):
```json
HTTP 200 OK
{
  "components": [
    {
      "name": "k0s",
      "status": {"state": "Running", "message": "Ready"}
    }
  ],
  "logs": "Installation logs...",
  "status": {"state": "Running", "message": "Infrastructure ready"}
}
```

**Response** (Validation Failure):
```json
HTTP 400 Bad Request
{
  "statusCode": 400,
  "message": "preflight checks failed"
}
```

### Get Infrastructure Status

**Endpoint**: `GET /api/install/infra/status`

Returns the same `Infra` response structure as the setup endpoint, providing consistent API behavior.

## Implementation Details

### Validation Controller

The `SetupInfra` function consolidates all validation logic:

```go
func (c *InstallController) SetupInfra(ctx context.Context, ignorePreflightFailures bool) (preflightsWereIgnored bool, err error)
```

**Validation Steps**:
1. Retrieve current preflight status
2. Check if preflights completed (success or failure)
3. Apply validation rules based on preflight state, CLI configuration, and user intent
4. If proceeding despite failures, report bypass metrics
5. Proceed with infrastructure setup if validation passes

### Error Handling

The system uses a predefined error variable for consistency:
```go
var ErrPreflightChecksFailed = errors.New("preflight checks failed")
```

This ensures consistent error messages across the application and simplifies testing.

### Metrics Reporting

The system reports metrics for preflight bypass scenarios:
- **Bypassed Preflights**: When preflights fail but installation proceeds (with CLI flag + user confirmation)
- **Failed Preflights**: Blocking scenarios are handled elsewhere in the system

### Configuration

**CLI Flag**: `--ignore-host-preflights`
- Must be specified when starting the installer
- Enables the capability to bypass preflight failures
- Does not automatically bypass - user confirmation still required

**API Configuration**: `allowIgnoreHostPreflights`
- Internal boolean reflecting CLI flag state
- Used by validation logic and exposed to frontend via preflight status responses

## Error Handling

### Validation Errors
- **HTTP 400**: Preflight validation failures (user-correctable)
- **HTTP 500**: System errors during validation (infrastructure issues)

### Error Messages
- Clear, actionable error messages using standardized error variables
- Consistent formatting across all validation scenarios
- No sensitive information exposure

## Security Considerations

### Threat Mitigation

**Unauthorized Bypass Prevention**:
- CLI flag requirement prevents runtime bypass without explicit operator intent
- API-level validation ensures frontend cannot bypass restrictions

**Accidental Override Prevention**:
- User must explicitly confirm override in UI
- Clear distinction between "proceed normally" and "override failures"

**Audit and Monitoring**:
- All validation decisions logged
- Metrics reported for bypass scenarios to enable monitoring
- Clear error messages for troubleshooting
- Consistent behavior for security review

### Best Practices

- Use `--ignore-host-preflights` only in emergency situations
- Understand risks before bypassing preflight checks
- Monitor systems where preflights were bypassed more closely

## Frontend Integration

### User Experience

**Normal Flow** (Preflights Pass):
1. User clicks "Continue" button
2. Frontend sends `ignorePreflightFailures: false`
3. Setup proceeds immediately

**Override Flow** (Preflights Fail):
1. User sees preflight failures
2. If CLI flag enabled, "Continue Anyway" button appears
3. User clicks button, sees confirmation modal
4. User confirms, frontend sends `ignorePreflightFailures: true`
5. Setup proceeds with override

### API Communication

The frontend receives CLI flag status via the preflight status endpoint:
```json
{
  "allowIgnoreHostPreflights": true,
  "status": {"state": "Failed"},
  ...
}
```

This enables appropriate UI state management and button visibility.

## Examples

### Successful Override Scenario

```bash
# 1. Start installer with bypass capability
./embedded-cluster install --ignore-host-preflights

# 2. Preflights fail, user confirms override
# 3. API call:
POST /api/install/infra/setup
{
  "ignorePreflightFailures": true
}

# 4. Response:
HTTP 200 OK
{
  "components": [...],
  "status": {"state": "Running"}
}
```

### Blocked Installation Scenario

```bash
# 1. Start installer without bypass capability
./embedded-cluster install

# 2. Preflights fail, API call:
POST /api/install/infra/setup
{
  "ignorePreflightFailures": false
}

# 3. Response:
HTTP 400 Bad Request
{
  "statusCode": 400,
  "message": "preflight checks failed"
}
```

## Related Components

### Backend
- `api/types/infra.go` - Request/response types
- `api/install.go` - HTTP handlers
- `api/controllers/install/infra.go` - Validation and setup logic

### Frontend
- `web/src/components/wizard/ValidationStep.tsx` - Preflight UI component

### Configuration
- `cmd/installer/cli/install.go` - CLI flag definition
- `cmd/installer/cli/api.go` - Configuration wiring

---

*This document describes the implementation completed as part of the infrastructure setup preflight validation feature.* 