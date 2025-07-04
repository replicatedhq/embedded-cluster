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
