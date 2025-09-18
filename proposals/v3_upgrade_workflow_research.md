# V3 Upgrade Workflow Research

## Executive Summary
This document provides comprehensive research on the existing codebase to understand how to implement a v3 upgrade workflow similar to the v3 install workflow. The research covers the current implementation of the v3 install workflow, existing upgrade mechanisms, API patterns, UI components, and state management.

## Current V3 Install Workflow

### Overview
The v3 install workflow is enabled via the `ENABLE_V3` environment variable and provides a manager UI experience for installations on both Linux and Kubernetes targets.

### Key Components

#### 1. CLI Layer (`cmd/installer/cli/install.go`)
- **Entry Point**: `InstallCmd` function checks `isV3Enabled()` to determine if v3 workflow is active
- **Manager Experience**: Triggered via `runManagerExperienceInstall` when `enableManagerExperience` flag is set
- **Target Support**: Supports both Linux (`--target=linux`) and Kubernetes (`--target=kubernetes`) targets
- **Authentication**: Generates or uses provided TLS certificates for secure communication

#### 2. API Layer (`api/`)

##### Routes (`api/routes.go`)
The API provides separate route groups for Linux and Kubernetes installations:

**Linux Routes** (`/linux/install/`):
- `/installation/config` - GET installation configuration
- `/installation/configure` - POST to configure installation
- `/installation/status` - GET installation status
- `/host-preflights/run` - POST to run host preflights
- `/host-preflights/status` - GET host preflight status
- `/app-preflights/run` - POST to run app preflights
- `/app-preflights/status` - GET app preflight status
- `/infra/setup` - POST to setup infrastructure
- `/infra/status` - GET infrastructure status
- `/app/config/template` - POST to template app config
- `/app/config/values` - GET/PATCH app config values
- `/app/install` - POST to install app
- `/app/status` - GET app installation status

**Kubernetes Routes** (`/kubernetes/install/`):
- Similar structure but without host-preflight endpoints
- Focused on app installation within existing Kubernetes cluster

##### State Machine (`api/internal/statemachine/`)
The installation process uses a state machine with the following states:
- `StateNew` - Initial state
- `StateApplicationConfiguring` / `StateApplicationConfigured`
- `StateInstallationConfiguring` / `StateInstallationConfigured`
- `StateHostConfiguring` / `StateHostConfigured` (Linux only)
- `StateHostPreflightsRunning` / `StateHostPreflightsSucceeded` / `StateHostPreflightsFailed` (Linux only)
- `StateInfrastructureInstalling` / `StateInfrastructureInstalled`
- `StateAppPreflightsRunning` / `StateAppPreflightsSucceeded` / `StateAppPreflightsFailed`
- `StateAppInstalling` / `StateSucceeded` / `StateAppInstallFailed`

#### 3. UI Layer (`web/src/`)

##### Wizard Steps (`web/src/components/wizard/InstallWizard.tsx`)
The installation wizard follows these steps:

**Linux Target**:
1. Welcome Step - Login/authentication
2. Configuration Step - App configuration
3. Linux Setup Step - Infrastructure settings
4. Installation Step - Actual installation process
5. Linux Completion Step - Success/completion

**Kubernetes Target**:
1. Welcome Step - Login/authentication
2. Configuration Step - App configuration
3. Kubernetes Setup Step - K8s specific settings
4. Installation Step - Actual installation process
5. Kubernetes Completion Step - Success/completion

##### Key UI Components
- `WelcomeStep` - Handles authentication and initial setup
- `ConfigurationStep` - Manages app configuration values
- `InstallationStep` - Shows installation progress with timeline
- `InstallationTimeline` - Visual progress indicator
- Phase components for preflights and installation

## Current Upgrade Mechanisms

### Traditional Upgrade (`cmd/installer/cli/update.go`)
- **Command**: `update` command with `--airgap-bundle` flag
- **Process**: 
  1. Validates airgap bundle matches binary
  2. Gets latest installation from cluster
  3. Calls `kotscli.AirgapUpdate` to perform update
- **Limitation**: CLI-only, no UI/manager experience

### KOTS CLI Integration (`cmd/installer/kotscli/kotscli.go`)
- **AirgapUpdate Function**: Handles airgap bundle updates
- **Install Function**: Handles initial app installation with config values
- **Key Features**:
  - Supports airgap and online modes
  - Config values via file
  - Preflight execution control
  - Progress masking for better UX

## API Architecture

### Manager Pattern
Each functional area has a manager interface:
- `AppInstallManager` - Manages app installation
- `AppConfigManager` - Handles app configuration
- `AppPreflightManager` - Manages app preflights
- `LinuxInfraManager` - Linux infrastructure setup
- `LinuxPreflightManager` - Linux host preflights

### Store Pattern
Persistent storage via store interfaces:
- `AppConfigStore` - Stores app configuration
- `AppPreflightStore` - Stores preflight results
- `LinuxInstallationStore` - Linux installation state
- `KubernetesInstallationStore` - K8s installation state

## Key Differences: Install vs Upgrade

### Installation Flow
1. **Fresh Start**: Begins with no existing infrastructure
2. **Full Setup**: Includes infrastructure provisioning
3. **Complete Configuration**: All config values must be provided
4. **Host Preflights**: Runs host system checks (Linux only)

### Upgrade Flow (Current)
1. **Existing Infrastructure**: Assumes cluster exists
2. **App-Only**: Updates only application, not infrastructure
3. **Minimal Configuration**: Reuses existing config
4. **No Host Checks**: Skips infrastructure validation

## Implementation Considerations

### 1. State Machine Modifications
For upgrades, we need a simplified state machine:
- Skip infrastructure-related states
- Focus on app configuration and update
- Potentially add upgrade-specific states (e.g., `StateAppUpgrading`)

### 2. API Endpoints
New endpoints needed under `/linux/upgrade/` and `/kubernetes/upgrade/`:
- `/upgrade/status` - Get current upgrade status
- `/upgrade/config` - Get/update app configuration
- `/upgrade/preflights/run` - Run upgrade preflights
- `/upgrade/preflights/status` - Get preflight status
- `/upgrade/execute` - Execute the upgrade
- `/upgrade/rollback` - Rollback if needed

### 3. UI Components
Reuse existing components with modifications:
- Skip setup step (no infrastructure changes)
- Modify installation step to show "Upgrading" instead of "Installing"
- Add upgrade-specific messaging and progress indicators

### 4. Configuration Handling
- **Existing Config**: Load and display current configuration
- **Config Changes**: Allow selective updates
- **Validation**: Ensure config changes are compatible

### 5. Preflight Considerations
- **App Preflights**: Run upgrade-specific checks
- **Skip Host Preflights**: Not needed for app-only upgrades
- **Version Compatibility**: Check upgrade path validity

## Technical Patterns

### Authentication Flow
1. User provides password at welcome step
2. API validates and returns JWT token
3. Token used for subsequent API calls
4. Session maintained throughout wizard

### Progress Tracking
- WebSocket or polling for real-time updates
- State machine provides current status
- Timeline UI shows visual progress
- Log streaming for detailed output

### Error Handling
- Each state can transition to error state
- Error messages propagated to UI
- Retry capabilities for recoverable errors
- Rollback options for critical failures

## Code Organization

### Directory Structure
```
api/
├── controllers/        # HTTP handlers
│   ├── app/           # App-related endpoints
│   ├── linux/         # Linux-specific endpoints
│   └── kubernetes/    # K8s-specific endpoints
├── internal/
│   ├── managers/      # Business logic
│   ├── statemachine/  # State management
│   └── store/         # Data persistence
└── types/            # Shared types

web/src/
├── components/
│   ├── wizard/       # Wizard steps
│   │   ├── config/   # Configuration components
│   │   ├── installation/ # Installation progress
│   │   └── completion/   # Completion screens
│   └── common/       # Shared UI components
├── contexts/         # React contexts
└── providers/        # Context providers
```

## Existing Upgrade Infrastructure

### KOTS Integration
- `kotscli.AirgapUpdate` - Existing upgrade function
- Supports airgap bundle updates
- Handles app version management
- Progress reporting via stdout

### Metrics Reporting
- Installation metrics via `metricsReporter`
- Tracks start, success, failure events
- Includes cluster and app metadata

## Security Considerations

### TLS/Certificate Management
- Self-signed cert generation for development
- Custom cert support via CLI flags
- Certificate validation in API layer

### Authentication
- Password-based login
- JWT token generation
- Session management
- API middleware for auth validation

## Testing Infrastructure

### Unit Tests
- Component tests for UI (`*.test.tsx`)
- Manager tests for business logic
- State machine transition tests

### Integration Points
- API integration tests
- End-to-end wizard flow tests
- State persistence validation

## Conclusion

The v3 upgrade workflow can leverage significant portions of the existing v3 install workflow infrastructure. Key areas requiring new development:

1. **New API endpoints** specific to upgrades
2. **Modified state machine** without infrastructure states
3. **UI adjustments** to skip setup step and show upgrade-specific messaging
4. **Configuration management** to handle existing vs new config values
5. **Upgrade-specific preflights** focused on compatibility

The existing patterns for authentication, progress tracking, error handling, and UI components can be largely reused, making the implementation straightforward with proper planning.