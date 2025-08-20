# Research: Direct Helm Chart Installation Without KOTS CLI

## Executive Summary
This research document analyzes the current implementation and identifies key areas for modifying the app install endpoint to install Helm charts directly while maintaining KOTS CLI for version record creation.

## Current Architecture

### 1. Installation Flow
The current installation flow follows this sequence:
1. `/kubernetes/install/app/install` endpoint receives request
2. `InstallController.InstallApp()` validates state transitions
3. `AppInstallManager.Install()` invokes KOTS CLI
4. KOTS CLI handles entire installation including:
   - Creating version records
   - Installing Helm charts
   - Managing application state

### 2. Key Components

#### AppInstallManager (`api/internal/managers/app/install/`)
- Primary interface for app installation
- Current implementation delegates entirely to KOTS CLI via `kotscli.Install()`
- Manages installation status and logging

#### KOTS CLI Integration (`cmd/installer/kotscli/kotscli.go`)
- `Install()` function executes kubectl-kots binary
- Passes configuration via command-line arguments
- Handles both online and airgap installations

#### Release Data (`pkg/release/release.go`)
- Contains `ReleaseData` struct with `HelmChartArchives [][]byte`
- Helm charts are already extracted and available in memory
- Also contains `HelmChartCRs [][]byte` for Helm chart custom resources

#### Helm Client (`pkg/helm/client.go`)
- Existing robust Helm client implementation
- Supports install, upgrade, uninstall operations
- Already used throughout the codebase for addon installation

## Technical Analysis

### 1. Available Helm Chart Data
The `ReleaseData` structure already contains:
```go
type ReleaseData struct {
    // ... other fields
    HelmChartCRs      [][]byte  // Helm chart custom resources
    HelmChartArchives [][]byte  // Actual Helm chart archives
}
```

### 2. Existing Helm Infrastructure
The codebase has a complete Helm client implementation that:
- Supports chart installation from archives
- Handles namespace creation
- Manages releases
- Provides timeout and retry logic

### 3. KOTS CLI Invocation Pattern
Current KOTS CLI invocation uses these key parameters:
- `--exclude-admin-console`: Already excludes admin console deployment
- `--app-version-label`: Version tracking
- `--config-values`: Application configuration
- `--skip-preflights`: Conditionally skip preflight checks

## Implementation Considerations

### 1. Dual Path Architecture
Need to implement:
- KOTS CLI path: Version record creation only (coordinated with sc-128049)
- Direct Helm path: Actual chart deployment

### 2. Chart Installation Requirements
- Extract charts from `HelmChartArchives`
- Apply configuration values
- Maintain proper installation order
- Handle dependencies

### 3. State Management
- Coordinate state transitions between KOTS and Helm operations
- Handle partial failures
- Implement rollback capabilities

### 4. Configuration Mapping
- Transform KOTS config values to Helm values
- Handle templating requirements
- Manage secrets and sensitive data

## Identified Challenges

### 1. Coordination with sc-128049
- Need to ensure KOTS CLI changes are compatible
- Timing of deployment between stories
- Feature flag or version detection strategy

### 2. Error Handling Complexity
- Dual-path failures (KOTS success, Helm failure)
- Partial installation states
- Recovery mechanisms

### 3. Backward Compatibility
- Support for existing installations
- Migration path for current deployments
- Feature detection and gradual rollout

### 4. Observability
- Logging across two systems
- Metrics collection
- Debugging capabilities

## Key Files to Modify

### Primary Changes
1. `api/internal/managers/app/install/install.go` - Core installation logic
2. `api/internal/managers/app/install/manager.go` - Manager interface updates
3. `cmd/installer/kotscli/kotscli.go` - KOTS CLI invocation modifications

### Supporting Changes
1. `api/internal/handlers/kubernetes/install.go` - Endpoint handlers
2. `api/controllers/app/install/install.go` - Controller logic
3. Configuration and test files

## Dependencies and Risks

### Dependencies
- Story sc-128049 (KOTS CLI modification)
- Existing Helm client functionality
- Release data structure

### Risks
1. **High Risk**: Dual deployment path complexity
2. **Medium Risk**: State synchronization issues
3. **Medium Risk**: Rollback complexity
4. **Low Risk**: Performance impact

## Recommendations

### 1. Phased Implementation
- Phase 1: Add Helm installation capability alongside KOTS
- Phase 2: Modify KOTS CLI invocation (with sc-128049)
- Phase 3: Full integration and testing

### 2. Feature Flag Strategy
- Implement feature flag for gradual rollout
- Allow fallback to original behavior
- Enable A/B testing in production

### 3. Comprehensive Testing
- Unit tests for new Helm installation logic
- Integration tests for dual-path scenario
- E2E tests for complete workflow
- Rollback scenario testing

### 4. Monitoring Strategy
- Add detailed logging at each step
- Implement metrics for success/failure rates
- Create dashboards for deployment monitoring

## Conclusion
The architecture supports this change with existing Helm infrastructure and available chart data. The primary complexity lies in coordinating the dual deployment paths and ensuring proper state management. A phased approach with feature flags and comprehensive testing will minimize risk.