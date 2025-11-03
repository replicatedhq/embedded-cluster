# Headless Install V3 Research

## Executive Summary

This document provides comprehensive research on the current embedded cluster codebase to inform the implementation of headless installs using the v3 API. The research covers the current architecture, authentication mechanisms, state management, and identifies key areas that need modification to support headless installs.

## Current Architecture

### V3 Installation Flow

The v3 installation uses a state machine pattern with the following key components:

1. **State Machine** (`api/internal/statemachine/`): Manages state transitions during installation
2. **API Server** (`api/`): RESTful API that provides endpoints for install operations
3. **Controllers** (`api/controllers/`): Business logic for different install targets (Linux/Kubernetes)
4. **Manager Experience UI** (`web/`): React-based UI that interacts with the API

### Installation States

The installation progresses through these states (from `api/internal/states/states.go`):

1. `StateNew` - Initial state
2. `StateApplicationConfiguring` - User configures application settings
3. `StateApplicationConfigured` - Application config complete
4. `StateInstallationConfiguring` - Installation settings configured
5. `StateInstallationConfigured` - Installation config complete
6. `StateHostConfiguring` - Host configuration
7. `StateHostConfigured` - Host configuration complete
8. `StateHostPreflightsRunning` - Running host preflights
9. `StateHostPreflightsSucceeded/Failed` - Host preflight results
10. `StateInfrastructureInstalling` - Installing infrastructure
11. `StateInfrastructureInstalled` - Infrastructure ready
12. `StateAirgapProcessing` - Processing airgap bundle (if airgap)
13. `StateAirgapProcessed` - Airgap bundle processed
14. `StateAppPreflightsRunning` - Running app preflights
15. `StateAppPreflightsSucceeded/Failed` - App preflight results
16. `StateAppInstalling` - Installing application
17. `StateSucceeded` - Installation complete

### Current V2 Headless Implementation

The existing v2 headless implementation (`cmd/installer/cli/install.go`):
- Uses `isHeadlessInstall := flags.configValues != "" && flags.adminConsolePassword != ""`
- Directly calls KOTS CLI without using the API
- Skips the UI entirely
- Does not use the state machine

## API Architecture

### Authentication

The v3 API uses JWT token-based authentication:

1. **Login** (`/api/auth/login`): Accepts password, returns JWT token
2. **Middleware**: All other endpoints require valid JWT in Authorization header
3. **Password Hash**: Stored as bcrypt hash, validated during login

### Key API Endpoints

#### Linux Install Endpoints
- `/api/linux/install/installation/configure` - Configure installation
- `/api/linux/install/host-preflights/run` - Run host preflights
- `/api/linux/install/infra/setup` - Setup infrastructure
- `/api/linux/install/airgap/process` - Process airgap bundle
- `/api/linux/install/app/config/values` - Get/patch config values
- `/api/linux/install/app-preflights/run` - Run app preflights
- `/api/linux/install/app/install` - Install application

#### Kubernetes Install Endpoints
Similar structure but without host-specific operations:
- `/api/kubernetes/install/installation/configure`
- `/api/kubernetes/install/app-preflights/run`
- `/api/kubernetes/install/infra/setup`
- `/api/kubernetes/install/app/config/values`
- `/api/kubernetes/install/app/install`

### State Transitions

Valid state transitions are enforced by the state machine. Key transitions for headless:
- Must acquire a lock before transitioning states
- Transitions are validated before execution
- Background operations release lock when complete

## Config Values Handling

### Current Implementation

1. **Validation** (`pkg/configutils/kots.go`):
   - Validates file exists and has correct GVK (`kots.io/v1beta1 ConfigValues`)

2. **Manager Experience**:
   - Accepts `configValues` in `APIConfig`
   - Validates and patches values during controller initialization
   - Uses `AppConfigManager` to handle values

3. **Template Functions**:
   - Config supports template functions (Distribution, VersionLabel, etc.)
   - Regex validation support exists for config items

## Key Challenges for Headless Mode

### 1. Authentication
- **Challenge**: API requires authentication token
- **Options**:
  - Generate token without user interaction
  - Bypass API and call managers directly
  - Create headless-specific endpoints without auth

### 2. State Machine Navigation
- **Challenge**: UI drives state transitions interactively
- **Solution Needed**: Automatic state progression in headless mode

### 3. Config Values Processing
- **Challenge**: Config values may have templating/validation errors
- **Solution Needed**: Clear error reporting without UI

### 4. Preflight Handling
- **Challenge**: Preflights may fail and require user decision
- **Solution Needed**: Honor bypass flags in headless mode

### 5. Error Reporting
- **Challenge**: No UI to show errors
- **Solution Needed**: Clear CLI output and exit codes

## Existing Infrastructure

### Relevant Components

1. **InstallCmdFlags** (`cmd/installer/cli/install.go`):
   - Already has `configValues` field
   - Has `assumeYes` flag for automation
   - Has `ignoreHostPreflights` and `ignoreAppPreflights` flags

2. **APIConfig** (`api/types/`):
   - Already accepts `ConfigValues`
   - Has all necessary configuration fields

3. **State Machine Event Handlers**:
   - Can register handlers for state transitions
   - Could auto-progress through states

## Code Locations

### Key Files to Modify

1. **CLI Layer**:
   - `cmd/installer/cli/install.go` - Add headless flag and logic
   - `cmd/installer/cli/flags.go` - Add new flags

2. **API Layer**:
   - `api/controllers/linux/install/controller.go` - Linux install controller
   - `api/controllers/kubernetes/install/controller.go` - Kubernetes install controller
   - `api/controllers/app/controller.go` - App installation logic

3. **State Machine**:
   - `api/internal/statemachine/statemachine.go` - Core state machine
   - `api/controllers/linux/install/statemachine.go` - Install state transitions

## Recommendations

### 1. API Integration Strategy

**Recommended Approach**: Use existing API with programmatic authentication

Rationale:
- Maintains single code path for install logic
- Leverages existing state machine
- Ensures consistency between UI and headless flows
- Easier to maintain long-term

Implementation:
- Generate auth token programmatically in headless mode
- Use HTTP client to call API endpoints
- Progress through states automatically

### 2. State Machine Auto-Progression

Register event handlers that automatically trigger next state transitions in headless mode:
- After host preflights succeed → start infrastructure install
- After infrastructure installed → process airgap (if needed)
- After airgap processed → run app preflights
- After app preflights → install app

### 3. Config Values Strategy

- Load and validate config values early
- Pass to API during initialization
- Surface templating/validation errors immediately
- Support both YAML and JSON formats

### 4. Error Handling

- Use structured logging for all operations
- Return specific exit codes for different failure types
- Provide clear error messages to stdout/stderr
- Support debug mode for verbose output

### 5. Preflight Handling

In headless mode:
- Run preflights automatically
- Honor `--ignore-host-preflights` and `--ignore-app-preflights` flags
- Fail fast on strict preflight failures
- Continue on bypassable failures if ignore flags are set

## Testing Considerations

### Unit Tests
- Test state machine transitions in headless mode
- Test config value loading and validation
- Test error scenarios

### Integration Tests
- Test full headless install flow
- Test with various config values
- Test preflight bypass scenarios
- Test airgap installations

### E2E Tests
- Test headless install on real infrastructure
- Test with CI/CD pipelines
- Test error recovery scenarios

## Security Considerations

1. **Authentication**: Headless mode still needs secure authentication
2. **Config Values**: May contain secrets, handle securely
3. **API Access**: Ensure API is only accessible locally during headless install
4. **Audit Trail**: Log all headless operations for security audit

## Performance Considerations

1. **Parallel Operations**: Some states could run in parallel
2. **Timeout Handling**: Need appropriate timeouts for each operation
3. **Progress Reporting**: Provide progress updates to CLI
4. **Resource Usage**: Monitor resource usage during headless installs

## Compatibility Notes

1. **V2 Compatibility**: New headless mode should not break existing v2 headless installs
2. **Flag Compatibility**: Maintain backward compatibility for existing flags where possible
3. **Config Format**: Support existing config value formats
4. **Exit Codes**: Use standard exit codes for scripting compatibility