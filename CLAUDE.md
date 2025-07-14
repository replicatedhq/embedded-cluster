# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Embedded Cluster is a platform by Replicated that allows you to distribute a Kubernetes cluster and your application together as a single appliance. It simplifies enterprise software deployment by consolidating all components into a single binary that handles streamlined cluster installation without external dependencies.

The project bundles k0s (open source Kubernetes distribution) with applications, providing a complete Kubernetes distribution that can be installed and managed as a single unit.

## Technology Stack

### Core Technologies
- **Go 1.24.4** - Primary language for backend, CLI, and operators
- **TypeScript/React** - Frontend web UI with Vite build system
- **Kubernetes (k0s v1.31.8+k0s.0)** - Foundation Kubernetes distribution
- **Docker** - Container runtime and development environment

### Key Dependencies
- **KOTS** - Application lifecycle management
- **Velero** - Backup and disaster recovery
- **OpenEBS** - Storage provisioner
- **SeaweedFS** - Distributed object storage for HA air gap mode
- **Helm** - Package management for Kubernetes applications

## Development Commands

### Essential Build Commands
```bash
# Create initial installation release
make initial-release

# Create upgrade release
make upgrade-release

# Set up development node
make create-node0

# Build with TTL.sh integration
make build-ttl.sh
```

### Testing Commands
```bash
# Run unit tests
make unit-tests

# Run end-to-end tests
make e2e-tests

# Code linting
make lint
```

### Build Targets
- `embedded-cluster-linux-amd64` - Linux AMD64 binary
- `embedded-cluster-linux-arm64` - Linux ARM64 binary
- `embedded-cluster-darwin-arm64` - macOS ARM64 binary

## Architecture Overview

### Core Design Patterns
- **Functional Options Pattern** - Standard for component initialization
- **Interface-Driven Design** - Behavior through interfaces for mocking/testing
- **Dependency Injection** - Decoupled components with clean interfaces
- **State Machine** - Enforces valid state transitions for installation workflows

### Key Components
- **Single Binary Distribution** - All components consolidated for easy deployment
- **Controller-Manager Pattern** - Controllers handle workflows, managers handle subdomains
- **Air Gap Support** - Complete offline installation capability
- **Custom Resource Definitions** - Installation, Config, KubernetesInstallation types

## Code Organization

### Primary Directories
- **`/cmd/installer/`** - Main CLI application with installation, join, reset, and management commands
- **`/cmd/local-artifact-mirror/`** - Local artifact mirror for air gap deployments
- **`/cmd/buildtools/`** - Build utilities and tooling

### API & Backend
- **`/api/`** - REST API server with controllers, handlers, and state management
  - Controllers for auth, console, kubernetes, and linux installation
  - Internal managers for domain-specific functionality
  - Swagger documentation generation

### Kubernetes Components
- **`/operator/`** - Kubernetes operator for managing cluster lifecycle
- **`/kinds/`** - Custom Resource Definitions (CRDs)
- **`/pkg/`** - Shared libraries and utilities
  - Addons (admin console, operator, storage, registry, etc.)
  - Network utilities, configuration management
  - Helm client, Kubernetes utilities

### Frontend & Testing
- **`/web/`** - React/TypeScript web UI with Tailwind CSS
- **`/e2e/`** - End-to-end integration tests
- **`/tests/`** - Unit and integration test suites
- **`/dev/`** - Development environment setup and tooling

## Development Guidelines

### Code Quality Standards
- **Clean Code Principles** - Concise comments, proper file formatting
- **Structured Error Handling** - Consistent error wrapping and context
- **Go Best Practices** - Proper error handling, naming conventions, interface design
- **API Guidelines** - Structured errors, consistent HTTP patterns

### Testing Requirements
- Unit tests alongside source files (`*_test.go`)
- Table-driven tests with `testify/assert`
- Mock all external dependencies
- Integration tests in dedicated directories

### Architecture Decisions
1. **Release Metadata Independence** - API doesn't depend on CLI embedded metadata
2. **Kubernetes as Linux Subset** - Kubernetes installations are subset of Linux installations
3. **Interface-Driven Design** - All components use interfaces for testability

## Local Development Environment

### Requirements
- macOS with Docker Desktop
- Go 1.24.4+
- Various CLI tools (helm, aws, kubectl, etc.)
- Development uses LXD containers for multi-node testing

### Development Flow
1. Set environment variables for Replicated API access
2. Create initial release with `make initial-release`
3. Spin up development nodes with `make create-node0`
4. Install using generated binary with license
5. Access admin console at localhost:30000

## Key Custom Resources

The system uses several custom Kubernetes resources:
- **Installation** - Tracks cluster and application upgrades
- **Config** - Runtime configuration management
- **ClusterConfig** - k0s configuration ingestion
- **Plan** - Autopilot operator configuration for upgrades
- **Chart** - Helm chart tracking and management