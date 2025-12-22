# V3 E2E Tests - Research Document

## Current State Analysis

### Existing E2E Test Infrastructure

#### Test Location and Organization
- E2E tests are located in `/e2e/` directory
- Tests are written in Go using standard Go testing framework
- Tests use Docker containers and LXD for test environments
- Playwright tests in `/e2e/playwright/` for UI testing

#### Current Test Execution Model
- Tests run via `make e2e-test TEST_NAME=TestSomething`
- GitHub Actions workflow orchestrates test execution in CI
- **Test environments use hybrid approach**:
  - Docker containers (`e2e/cluster/docker/`) for lightweight tests
  - LXD VMs (`e2e/cluster/lxd/`) for some VM-based tests
  - CMX VMs (`e2e/cluster/cmx/`) for advanced scenarios
- Tests are executed in parallel in CI with fail-fast disabled
- **Problem**: Multiple VM provisioning approaches create maintenance burden and inconsistency

#### Problems with V2 E2E Tests
1. **Not Portable**: Tests are tightly coupled to GitHub Actions workflows
2. **Complex Secret Management**: Multiple secrets needed across different services
3. **CI-Embedded Setup**: Much of the test setup is embedded in CI YAML files
4. **Difficult Local Execution**: Running tests locally requires mimicking CI environment
5. **No Build/Test Separation**: Tests rebuild artifacts each time they run
6. **Hybrid VM Approach**: Using Docker, LXD, and CMX creates:
   - Inconsistent test environments
   - Multiple codepaths to maintain
   - Different behavior between test types
   - LXD is not portable (Linux-specific, requires specific setup)

### V3 Installer Architecture

#### V3 Install Features
- Located in `cmd/installer/cli/install_v3.go`
- Supports both headless and browser-based installations
- Uses orchestrator pattern (`cmd/installer/cli/headless/install/orchestrator.go`)
- API-driven architecture with client/server model
- Uses HTTPS API on localhost with configurable ports

#### Headless Installation
- Orchestrator manages the installation workflow
- API client communicates with local API server
- No user interaction required for automated testing
- Supports signal handling and graceful shutdown

### Existing Dagger Usage

#### Current Dagger Setup
- Dagger configuration exists (`dagger.json`, `dagger/main.go`)
- Engine version: v0.19.2
- SDK: Go
- Currently used for building chainguard images
- Not yet integrated with E2E testing

### Secret Management

#### Current Approach
- GitHub Actions secrets for CI
- Environment variables for local development
- Secrets include:
  - AWS credentials for S3 operations
  - DockerHub credentials
  - License IDs for test applications
  - Staging environment credentials

#### Problems
- Secrets scattered across multiple systems
- No centralized secret management
- Difficult to share secrets across team members
- Local development requires manual secret configuration

### CI/CD Integration

#### Current GitHub Actions Workflow
- Jobs: `should-run-e2e`, `e2e-docker`, `e2e`, `e2e-main`
- Conditional execution based on file changes
- Complex dependency chain between jobs
- Matrix strategy for parallel test execution
- Embedded setup logic in workflow YAML

## Key Findings

### What Needs Testing in V3

1. **Installation Flows**
   - Headless installation via CLI
   - Browser-based installation via UI
   - Configuration validation
   - Error handling and recovery

2. **API Functionality**
   - Authentication
   - Installation orchestration
   - Status reporting
   - Error responses

3. **Upgrade Scenarios**
   - V2 to V3 upgrades
   - V3 to V3 upgrades
   - Rollback capabilities

4. **Multi-node Operations**
   - Controller node join
   - Worker node join
   - Node removal
   - High availability setup

### Gaps in Current Testing

1. No V3 smoke tests exist
2. No way to validate V3 works in key scenarios:
   - Online installations
   - Airgap installations
   - HTTP proxy environments
3. No portable test execution framework for V3
4. Cannot easily test V3 changes locally

### Requirements for V3 E2E Tests

Based on the Shortcut story and codebase analysis:

1. **E2E tests**: 3 scenarios × 2 modes = 6 tests (not comprehensive coverage)
   - Online, airgap, HTTP proxy
   - Each scenario tests both browser-based and headless installation modes
2. **Dual installation modes**:
   - Browser-based tests use Playwright
   - Headless tests use CLI/API (no Playwright)
   - Both modes validate using Kube client
3. **Portability**: Tests must run identically locally and in CI
4. **Dagger Integration**: Use Dagger for test orchestration
5. **1Password Integration**: Centralized secret management
6. **Build Separation**: Build once, test multiple times
7. **Local Development**: Easy local test execution without CI dependencies
8. **CMX Exclusive**: All tests use CMX (no Docker/LXD)
9. **V2 Unchanged**: Do not modify V2 tests

### Technical Considerations

1. **Dagger Benefits**
   - Consistent execution environment
   - Cached builds and dependencies
   - Portable across local and CI
   - Language-agnostic orchestration

2. **1Password Integration Options**
   - Dagger onepassword module
   - Environment variable injection
   - Secret rotation support
   - Team-shared vaults

3. **Test Organization Strategy**
   - Separate test suites for headless vs UI
   - Modular test helpers and utilities
   - Reusable test fixtures
   - Clear test naming conventions

4. **Build/Test Separation**
   - Build artifacts in Dagger pipeline
   - Cache built artifacts
   - Reference artifacts in test runs
   - Version tagging for artifact tracking

## Additional Findings from Deep Analysis

### Current Test Execution Details
- **Docker-based tests**: Run in Ubuntu 22.04 runners
- **Test matrix includes**:
  - TestPreflights, TestSingleNodeInstallation
  - TestMultiNodeInstallation, TestSingleNodeDisasterRecovery
  - TestSingleNodeAirgapUpgradeSelinux
  - TestCollectSupportBundle
- **Environment setup requires**:
  - Kernel modules (overlay, ip_tables, br_netfilter, nf_conntrack)
  - Docker login to avoid rate limiting
  - AWS credentials for S3 operations
  - Free disk space actions

### Playwright Test Structure
- **Test directories**:
  - create-backup, deploy-app, deploy-upgrade
  - get-join-controller-commands, get-join-worker-commands
  - login-with-custom-password, validate-restore-app
- **Configuration**: Uses Chromium only, baseURL defaults to http://localhost:30000
- **Reporting**: HTML reports with trace/screenshot on failure

### Build and Release Process
- **Build orchestration**: `scripts/build-and-release.sh` is the main entry point
  - Calls `scripts/ci-build-deps.sh` - Builds dependencies
  - Calls `scripts/ci-build-bin.sh` - Builds embedded-cluster binaries
  - Calls `scripts/ci-embed-release.sh` - Embeds release metadata
  - Calls `scripts/ci-upload-binaries.sh` - Uploads binaries to S3
  - Calls `scripts/ci-release-app.sh` - Creates app release in Replicated SaaS
- **Release YAML directories**:
  - `e2e/kots-release-install-v3/` for V3 installs (contains test app fixtures)
  - `e2e/kots-release-upgrade-v3/` for V3 upgrades
- **Artifact management**: Uses S3 for binary storage
- **Version management**: EC_VERSION, APP_VERSION, K0S_VERSION
- **App release creation**: Scripts create releases in embedded-cluster-smoke-test-staging-app
- **Test fixtures**: YAML files in `e2e/kots-release-install-v3/` define test application
- **Current CI usage**: GitHub Actions runs `scripts/build-and-release.sh` to build and release

### Existing Dagger Modules
- **chainguard.go**: Builds Chainguard images
- **localartifactmirror.go**: Manages local artifact mirror
- **operator.go**: Builds operator components
- **main.go**: Main Dagger module entry point

## Recommendations

1. **Build E2E test framework**: 6 tests total (3 scenarios × 2 modes) - not comprehensive
   - Each scenario (online, airgap, HTTP proxy) tests both browser-based and headless modes
2. **Adopt Dagger**: Leverage existing Dagger setup and extend for V3 E2E testing
3. **Implement 1Password**: Use onepassword Dagger module for secret management
4. **Create separate V3 framework**: New framework independent of V2 tests
5. **Test both installation modes**:
   - Browser-based tests: Use Playwright for UI workflows
   - Headless tests: Use CLI/API (no Playwright)
   - Both modes: Validate using Kube client
6. **Keep it simple**: Minimal infrastructure for focused smoke testing
7. **Reuse Playwright**: Wrap existing Playwright tests with Dagger for browser-based tests
8. **Extract CI Logic**: Move setup from GitHub Actions YAML to Dagger modules
9. **Wrap Existing Scripts**: Use Dagger to wrap `scripts/build-and-release.sh` and related scripts rather than rewriting in Go
   - **Rationale**: These scripts are battle-tested and handle complex orchestration
   - **Benefit**: Reduces implementation risk and time
   - **Approach**: Create Dagger containers that execute the existing bash scripts
   - **Scripts to wrap**: ci-build-deps.sh, ci-build-bin.sh, ci-embed-release.sh, ci-upload-binaries.sh, ci-release-app.sh
10. **Standardize on CMX for ALL V3 Tests**: Use CMX exclusively for the new V3 test framework
    - **Current problem**: V2 uses hybrid Docker/LXD/CMX approach creating maintenance burden
    - **V3 approach**: CMX for all test scenarios (no Docker, no LXD)
    - **CMX benefits**:
      - Portable across platforms (not Linux-specific like LXD)
      - Supports advanced scenarios (airgap, proxy testing)
      - Consistent test environment across all tests
      - Already integrated with existing infrastructure
      - Handles both lightweight and full VM scenarios
    - **V2 tests**: Left completely unchanged
      - Will continue to use existing Docker/LXD/CMX hybrid approach
      - Will be deprecated once V3 upgrade path exists
      - Eventually removed when V2 installer is retired
    - **No migration**: V2 tests are NOT being migrated to V3 framework