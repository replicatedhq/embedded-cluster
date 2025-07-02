# Installation Config vs Runtime Config

This document explains the key architectural differences between Installation Config and Runtime Config in the Embedded Cluster project.

## Overview

The project uses two primary configuration types that work in tandem but serve fundamentally different purposes:

**Installation Config** → **Runtime Config**

Understanding this distinction is crucial for proper configuration management in the system.

## Core Differences

### Installation Config (`api/types/installation.go`)
**Purpose**: User-configurable installation parameters  
**Scope**: Installation-specific settings that affect cluster setup  
**Lifecycle**: Created during installation, exists only during installation process  
**Persistence**: Stored in memory via `installation.Store` interface during installation  
**Owner**: User/Installation Process  
**Nature**: Source of truth for user preferences

### Runtime Config (`pkg/runtimeconfig/`)
**Purpose**: System runtime configuration and environment management  
**Scope**: Process environment, file paths, and runtime behavior  
**Lifecycle**: Lives for the entire cluster lifetime  
**Persistence**: Persisted to disk at `/etc/embedded-cluster/ec.yaml`  
**Owner**: System/Cluster Runtime  
**Nature**: Derived state from installation preferences

## Key Architectural Distinctions

### 1. Source vs. Derived State

**Installation Config**:
- Contains user-provided values and choices
- Represents "what the user wants"
- Input to the system
- Source of truth for user preferences

**Runtime Config**:
- Contains computed values and system paths
- Represents "how the system is configured"
- System state representation
- Derived from user preferences

### 2. Lifecycle and Persistence Strategy

**Installation Config**:
- Created during installation process
- Lives in memory during installation
- Discarded after installation completes
- Temporary, process-specific storage

**Runtime Config**:
- Created from installation config
- Persisted to disk for durability
- Lives for entire cluster lifetime
- Survives restarts and upgrades

### 3. Data Flow Direction

```
User Input → Installation Config → Runtime Config → System Environment
```

The data flows in one direction only:
- User provides input to Installation Config
- Installation Config validates and stores user preferences
- Runtime Config receives validated preferences and computes system state
- System environment is configured from Runtime Config

### 4. Validation Responsibilities

**Installation Config**:
- User-facing validation with clear error messages
- Business rule validation (port conflicts, network ranges)
- Input format validation (required fields, valid formats)
- Cross-field validation to ensure configuration coherence

**Runtime Config**:
- System constraint validation (path permissions, disk space)
- Environment validation (required directories exist)
- Internal consistency checks for derived values

## Detailed Comparison

### Network Configuration Handling

**Installation Config**:
- Stores user-specified port preferences
- Handles network interface selection
- Manages CIDR range choices
- Validates port availability and network conflicts

**Runtime Config**:
- Provides access to configured ports via getter methods
- Manages environment variables for network configuration
- Handles system-level network configuration
- Applies network settings to the running system

### Directory and Path Management

**Installation Config**:
- Stores user preference for high-level directory location
- Validates directory accessibility and permissions
- Represents user choice for data storage location

**Runtime Config**:
- Computes all specific system paths based on user preference
- Manages file and directory locations for all cluster components
- Provides path accessors for kubeconfig, K0s config, etc.
- Handles path creation and management

### Configuration Updates and Synchronization

**Installation Config Flow**:
1. User provides configuration via API/UI
2. Configuration is validated for user-facing constraints
3. Configuration is stored in memory during installation
4. Configuration drives runtime config updates

**Runtime Config Flow**:
1. Receives updates from validated installation config
2. Computes derived values and system paths
3. Updates environment variables
4. Persists configuration to disk for durability

## Data Transformation Examples

### Port Configuration
- **Installation Config**: User specifies `AdminConsolePort: 30000`
- **Runtime Config**: Sets `ADMIN_CONSOLE_PORT=30000` in environment and provides `AdminConsolePort()` accessor

### Directory Configuration
- **Installation Config**: User specifies `DataDirectory: "/opt/my-cluster"`
- **Runtime Config**: Computes all system paths:
  - `EmbeddedClusterHomeDirectory()` → `"/opt/my-cluster"`
  - `PathToKubeConfig()` → `"/opt/my-cluster/k0s/pki/admin.conf"`
  - `K0sConfigPath()` → `"/opt/my-cluster/k0s/k0s.yaml"`

## Architectural Patterns

### Separation of Concerns
- **Installation Config**: Handles user interface and input validation
- **Runtime Config**: Manages system state and environment
- Clear boundaries prevent mixing of responsibilities

### Single Direction Data Flow
- Data flows from Installation Config to Runtime Config only
- No reverse dependencies or circular updates
- Clear transformation pipeline from user input to system state

### Appropriate Persistence Strategy
- **Installation Config**: Temporary, in-memory during installation
- **Runtime Config**: Permanent, disk-based for cluster lifetime
- Each uses persistence mechanism appropriate to its lifecycle

### Validation at Boundaries
- User input validated in Installation Config layer
- System constraints validated in Runtime Config layer
- No duplication of validation logic across layers

### Error Handling Strategy
- **Installation Config**: User-friendly error messages with actionable guidance
- **Runtime Config**: System-level error messages for operational issues
- Structured error types for consistent error handling

## Integration Architecture

### Controller Integration Pattern
The typical flow in controllers follows this pattern:

1. **Validate user input** using Installation Config validation
2. **Store installation config** temporarily in memory
3. **Transform to runtime config** by updating system state
4. **Apply environment changes** via Runtime Config
5. **Persist runtime state** to disk for durability

This pattern ensures proper separation of concerns and maintains data flow integrity.

### Configuration Synchronization
- Installation Config and Runtime Config are kept synchronized during installation
- Environment variables are updated after runtime config changes
- Configuration changes are persisted atomically
- Partial update failures are handled gracefully
