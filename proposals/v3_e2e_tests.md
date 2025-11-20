# V3 E2E Tests Implementation Proposal

## TL;DR

We will implement a portable, Dagger-based E2E test framework exclusively for V3 installer for testing **both headless and browser-based installation modes**. The framework enables local development and removes CI-embedded setup. It will use 1Password for secret management, separate build from test execution, and run identically locally and in CI. Tests run concurrently using isolated CMX VMs to reduce total runtime. All tests will use CMX exclusively. Browser-based tests use Playwright. Both modes validate installation success using the Kube client. Existing build scripts will be wrapped in Dagger rather than rewritten. **V2 tests remain completely unchanged**. Comprehensive coverage is provided by unit and integration tests; E2E tests focus on validating external dependencies in the most common, happy path scenarios.

## The Problem

There are no E2E tests for V3. Smoke testing must be done manually before release.

We have unit and integration tests with comprehensive coverage, but nothing testing external dependencies. E2E tests are needed to validate V3 works in real-world deployment scenarios with external dependencies.

**Scope**: E2E tests should remain minimal - just validating external dependencies work. Unit and integration tests provide comprehensive coverage of business logic.

**Impact**: Manual smoke testing before each release is time-consuming and error-prone. Developers cannot easily validate V3 changes work with external dependencies.

## Prototype / Design

### High-Level Architecture

```
┌────────────────────────────────────────────────────────┐
│                   Dagger CLI Interface                 │
├────────────────────────────────────────────────────────┤
│                                                        │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  │
│  │   Build      │  │  Test Suite  │  │   Secrets    │  │
│  │   Module     │  │   Modules    │  │   Module     │  │
│  ├──────────────┤  ├──────────────┤  ├──────────────┤  │
│  │ - Binaries   │  │ - UI Tests   │  │ - 1Password  │  │
│  │ - Images     │  │ - Headless   │  │ - Env Vars   │  │
│  │ - Artifacts  │  │              │  │ - Creds      │  │
│  └──────────────┘  └──────────────┘  └──────────────┘  │
│                                                        │
│  ┌──────────────────────────────────────────────────┐  │
│  │             Test Execution Engine                │  │
│  ├──────────────────────────────────────────────────┤  │
│  │ - Environment provisioning (CMX VMs)             │  │
│  │ - Test orchestration                             │  │
│  │ - Result aggregation                             │  │
│  │ - Artifact management                            │  │
│  └──────────────────────────────────────────────────┘  │
└────────────────────────────────────────────────────────┘
```

### Data Flow

1. Developer/CI invokes Dagger CLI with test parameters
2. Secrets module fetches credentials from 1Password
3. Build module creates artifacts
4. Test suite modules execute tests in parallel, each in isolated CMX VMs
5. Results are aggregated and reported

### V3 Smoke Test Scenarios

The V3 test framework will implement exactly **2 smoke test scenarios**, each testing **both headless and browser-based installation modes**:

1. **Online Installation E2E Test**
   - **Browser-based**: Fresh V3 installation with internet connectivity using UI (Playwright tests)
   - **Headless**: Fresh V3 installation via CLI (no Playwright)
   - Both modes validate installation success using Kube client

2. **Airgap Installation E2E Test**
   - **Browser-based**: V3 airgap installation using UI workflow (Playwright tests)
   - **Headless**: V3 airgap installation via CLI (no Playwright)
   - Both modes validate installation success using Kube client

**Total test runs**: 4 (2 scenarios × 2 installation modes)

**Not in scope**: HTTP proxy, multi-node, upgrades, disaster recovery, comprehensive feature testing (covered by unit and integration tests)

## Goals & Non-Goals

### Goals
1. **E2E test V3 installer** in 2 key scenarios × 2 installation modes = 4 test runs:
   - Online (browser-based + headless)
   - Airgap (browser-based + headless)
2. **Test both installation modes**: Browser-based (with Playwright) and headless (CLI, no Playwright)
3. **Validate using Kube client**: Both modes use Kube client to validate installation success
4. **Enable local testing** of V3 changes without CI dependencies
5. **Portable execution** - same tests run locally and in CI
6. **Parallel execution** - tests must be runnable concurrently in isolation to reduce total runtime
7. **Simple, maintainable** - minimal test infrastructure, easy to understand

### Non-Goals
1. **No comprehensive testing** - Unit and integration tests provide comprehensive coverage; E2E tests only validate the most common, happy path scenarios
2. **Not testing HTTP proxy, multi-node, upgrades, disaster recovery** - We can defer the decision to augment our testing scenarios later to cover these
3. **Not migrating V2 tests** - V2 tests remain unchanged

## New Subagents / Commands

No new subagents or commands will be created as part of this implementation. This proposal focuses on test infrastructure only.

## Database

No database changes required.

## Implementation Plan

### Files and Services to Touch

#### New Files/Directories
- `dagger/e2e/` - Main E2E test Dagger modules
  - `dagger/e2e/build.go` - Build module for artifacts
  - `dagger/e2e/secrets.go` - 1Password integration module
  - `dagger/e2e/ui_tests.go` - UI test execution module
  - `dagger/e2e/headless_tests.go` - Headless test execution module
  - `dagger/e2e/validation.go` - Kubernetes validation helpers
  - `dagger/e2e/utils.go` - Shared utilities

- `e2e/v3/` - V3-specific test implementations
  - `e2e/v3/ui/` - Browser-based UI tests
  - `e2e/v3/ui/playwright.config.ts` - Playwright config for V3 tests
  - `e2e/v3/headless/` - Headless installation tests
  - `e2e/v3/fixtures/` - Test data and configuration

#### Modified Files
- `dagger/main.go` - Add E2E test commands
- `.github/workflows/ci.yaml` - Simplify to use Dagger commands

### Pseudo Code

#### Build Module
```go
// dagger/e2e/build.go
// Note: This wraps existing build-and-release.sh script rather than reimplementing in Go
func (m *BuildModule) BuildArtifacts(ctx context.Context) (*Artifacts, error) {
    // Create container with build environment
    builder := dag.Container().
        From("ubuntu:22.04").
        WithDirectory("/src", m.source).
        WithWorkdir("/src").
        WithExec([]string{"apt-get", "update"}).
        WithExec([]string{"apt-get", "install", "-y", "make", "git", "curl"})

    // Set environment variables
    builder = builder.
        WithEnvVariable("EC_VERSION", m.version).
        WithEnvVariable("APP_VERSION", m.appVersion).
        WithEnvVariable("RELEASE_YAML_DIR", "e2e/kots-release-install-v3").
        WithSecretVariable("AWS_ACCESS_KEY_ID", m.secrets.Get("aws_access_key")).
        WithSecretVariable("AWS_SECRET_ACCESS_KEY", m.secrets.Get("aws_secret_key"))

    // Run existing build script (battle-tested, don't rewrite)
    builder = builder.WithExec([]string{"./scripts/build-and-release.sh"})

    // Extract built artifacts
    binary := builder.File("output/bin/embedded-cluster")

    artifacts := &Artifacts{
        Binary: binary,
        Version: m.version,
    }

    return artifacts, nil
}
```

#### Test Execution Module
```go
// dagger/e2e/tests.go

// RunBrowserBasedTest runs a browser-based installation test with Playwright.
func (m *TestModule) RunBrowserBasedTest(
    ctx context.Context,
    // Installation scenario to test. Valid values: "online", "airgap"
    scenario string,
) (*TestResults, error) {
    // Get secrets from 1Password
    secrets := m.secrets.GetBatch(ctx, []string{
        "replicated_api_token",
        "cmx_api_token",
    })

    // Provision CMX VM for browser based installation test
    vm := m.provisionCMXVM(ctx, CMXConfig{
        OS:       "ubuntu-22.04",
        Memory:   "8GB",
        CPUs:     4,
        Secrets:  secrets,
        Scenario: scenario, // "online", "airgap"
    })
    defer vm.Cleanup()

    // Run browser based Playwright installation
    result := m.installBrowserBased(ctx, vm, PlaywrightConfig{
        Scenario:   scenario,
        AppVersion: m.appVersion,
        License:    m.getLicenseForScenario(scenario),
        LicenseID:  m.getLicenseIDForScenario(scenario),
    })

    // Validate installation succeeded using Kube client
    // Note: Both browser-based and headless tests use Kube client for validation
    // This performs comprehensive validation (see Installation Validation section)
    validationResults := m.validate(ctx, scenario, vm, result)

    return &TestResults{
        Scenario: scenario,
        Mode:     "browser-based",
        Success:  validationResults.Success,
        Details:  validationResults,
    }, nil
}

// RunHeadlessTest runs a headless (CLI) installation test without Playwright.
func (m *TestModule) RunHeadlessTest(
    ctx context.Context,
    // Installation scenario to test. Valid values: "online", "airgap"
    scenario string,
) (*TestResults, error) {
    // Get secrets from 1Password
    secrets := m.secrets.GetBatch(ctx, []string{
        "replicated_api_token",
        "cmx_api_token",
    })

    // Provision CMX VM for headless installation test
    vm := m.provisionCMXVM(ctx, CMXConfig{
        OS:       "ubuntu-22.04",
        Memory:   "8GB",
        CPUs:     4,
        Secrets:  secrets,
        Scenario: scenario, // "online", "airgap"
    })
    defer vm.Cleanup()

    // Run headless installation via CLI
    // Note: No Playwright for headless tests
    result := m.installHeadless(ctx, vm, HeadlessConfig{
        Scenario:   scenario,
        AppVersion: m.appVersion,
        License:    m.getLicenseForScenario(scenario),
        LicenseID:  m.getLicenseIDForScenario(scenario),
    })

    // Validate installation succeeded using Kube client
    // Note: Both browser-based and headless tests use Kube client for validation
    // This performs comprehensive validation (see Installation Validation section)
    validationResults := m.validate(ctx, scenario, vm, result)

    return &TestResults{
        Scenario: scenario,
        Mode:     "headless",
        Success:  validationResults.Success,
        Details:  validationResults,
    }, nil
}

// RunOnlineTests runs both browser-based and headless tests for online installation scenario.
func (m *TestModule) RunOnlineTests(ctx context.Context) error {
    // Run browser-based test
    browserResults, err := m.RunBrowserBasedTest(ctx, "online")
    if err != nil {
        return err
    }

    // Run headless test
    headlessResults, err := m.RunHeadlessTest(ctx, "online")
    if err != nil {
        return err
    }

    return m.reportResults(ctx, browserResults, headlessResults)
}

// RunAirgapTests runs both browser-based and headless tests for airgap installation scenario.
func (m *TestModule) RunAirgapTests(ctx context.Context) error {
    // Run browser-based test
    browserResults, err := m.RunBrowserBasedTest(ctx, "airgap")
    if err != nil {
        return err
    }

    // Run headless test
    headlessResults, err := m.RunHeadlessTest(ctx, "airgap")
    if err != nil {
        return err
    }

    return m.reportResults(ctx, browserResults, headlessResults)
}

// RunAllTests runs all 4 test combinations in parallel: online browser-based,
// online headless, airgap browser-based, and airgap headless.
func (m *TestModule) RunAllTests(ctx context.Context) error {
    var results []*TestResults

    // Run all tests in parallel
    errChan := make(chan error, 4)
    resultsChan := make(chan *TestResults, 4)

    tests := []struct {
        name     string
        scenario string
        fn       func(context.Context, string) (*TestResults, error)
    }{
        {"online-browser", "online", m.RunBrowserBasedTest},
        {"online-headless", "online", m.RunHeadlessTest},
        {"airgap-browser", "airgap", m.RunBrowserBasedTest},
        {"airgap-headless", "airgap", m.RunHeadlessTest},
    }

    for _, test := range tests {
        go func(t struct {
            name     string
            scenario string
            fn       func(context.Context, string) (*TestResults, error)
        }) {
            result, err := t.fn(ctx, t.scenario)
            if err != nil {
                errChan <- fmt.Errorf("%s failed: %w", t.name, err)
                return
            }
            resultsChan <- result
        }(test)
    }

    // Collect results
    for i := 0; i < 4; i++ {
        select {
        case err := <-errChan:
            return err
        case result := <-resultsChan:
            results = append(results, result)
        }
    }

    return m.reportAllResults(ctx, results)
}

// BuildAndRelease builds artifacts and creates a release
func (m *BuildModule) BuildAndRelease(ctx context.Context) (*Artifacts, error) {
    // Build artifacts using wrapped scripts
    artifacts, err := m.BuildArtifacts(ctx)
    if err != nil {
        return nil, fmt.Errorf("build failed: %w", err)
    }

    // Artifacts are uploaded to S3 and app release is created
    // as part of the build-and-release.sh script execution
    return artifacts, nil
}
```

#### Secret Management
```go
// dagger/e2e/secrets.go
func (m *SecretsModule) Initialize(ctx context.Context) error {
    // Connect to 1Password
    m.client = onepassword.NewClient(
        onepassword.WithVault("embedded-cluster-e2e"),
        onepassword.WithServiceAccount(os.Getenv("OP_SERVICE_ACCOUNT_TOKEN")),
    )

    // Validate connection
    return m.client.Validate(ctx)
}

func (m *SecretsModule) Get(ctx context.Context, key string) (string, error) {
    // Fetch from 1Password
    value, err := m.client.GetSecret(ctx, key)
    if err != nil {
        return "", err
    }

    return value, nil
}
```

#### Installation Methods
```go
// dagger/e2e/install.go

// downloadAndPrepareRelease downloads embedded-cluster release from replicated.app
// and prepares it for installation. This matches how customers get the binary.
func (m *TestModule) downloadAndPrepareRelease(ctx context.Context, vm *CMXInstance, appVersion, licenseID string) error {
    // Download embedded-cluster release from replicated.app
    downloadCmd := []string{
        "curl", "--retry", "5", "--retry-all-errors", "-fL",
        "-o", "/tmp/ec-release.tgz",
        fmt.Sprintf("https://ec-e2e-replicated-app.testcluster.net/embedded/embedded-cluster-smoke-test-staging-app/ci/%s", appVersion),
        "-H", fmt.Sprintf("Authorization: %s", licenseID),
    }
    if _, _, err := vm.RunCommand(ctx, downloadCmd); err != nil {
        return fmt.Errorf("failed to download release: %w", err)
    }

    // Extract release tarball
    if _, _, err := vm.RunCommand(ctx, []string{"tar", "xzf", "/tmp/ec-release.tgz", "-C", "/tmp"}); err != nil {
        return fmt.Errorf("failed to extract release: %w", err)
    }

    // Move binary and license to expected locations
    if _, _, err := vm.RunCommand(ctx, []string{"mkdir", "-p", "/assets"}); err != nil {
        return err
    }
    if _, _, err := vm.RunCommand(ctx, []string{"mv", "/tmp/embedded-cluster-smoke-test-staging-app", "/usr/local/bin/embedded-cluster-smoke-test-staging-app"}); err != nil {
        return err
    }
    if _, _, err := vm.RunCommand(ctx, []string{"mv", "/tmp/license.yaml", "/assets/license.yaml"}); err != nil {
        return err
    }
    if _, _, err := vm.RunCommand(ctx, []string{"chmod", "+x", "/usr/local/bin/embedded-cluster-smoke-test-staging-app"}); err != nil {
        return err
    }

    return nil
}

// installBrowserBased performs a browser-based installation using Playwright
func (m *TestModule) installBrowserBased(ctx context.Context, vm *CMXInstance, config PlaywrightConfig) (*InstallResult, error) {
    // Download and prepare embedded-cluster release
    if err := m.downloadAndPrepareRelease(ctx, vm, config.AppVersion, config.LicenseID); err != nil {
        return nil, err
    }

    // Start embedded-cluster install (headless mode to get to UI)
    // This starts the installation and makes the UI available on port 30000
    installCmd := []string{
        "/usr/local/bin/embedded-cluster-smoke-test-staging-app", "install",
        "--target", "linux",
        "--headless",
        "--yes",
    }

    if config.Scenario == "airgap" {
        installCmd = append(installCmd, "--airgap-bundle", "/tmp/airgap-bundle.tar.gz")
    }

    if _, _, err := vm.RunCommandAsync(ctx, installCmd); err != nil {
        return nil, err
    }

    // Wait for UI to be available
    if err := vm.WaitForPort(ctx, 30000, 2*time.Minute); err != nil {
        return nil, fmt.Errorf("UI did not become available: %w", err)
    }

    // Run Playwright tests to complete installation via UI
    // This includes: setting admin password, uploading license, deploying app
    playwrightResult, err := m.runPlaywrightInstallation(ctx, vm, config)
    if err != nil {
        return nil, fmt.Errorf("playwright installation failed: %w", err)
    }

    return &InstallResult{
        Success:        playwrightResult.Success,
        UIPort:         30000,
        KubeconfigPath: "/var/lib/embedded-cluster-smoke-test-staging-app/k0s/pki/admin.conf",
    }, nil
}

// installHeadless performs a headless (CLI/API) installation without Playwright
func (m *TestModule) installHeadless(ctx context.Context, vm *CMXInstance, config HeadlessConfig) (*InstallResult, error) {
    // Download and prepare embedded-cluster release
    if err := m.downloadAndPrepareRelease(ctx, vm, config.AppVersion, config.LicenseID); err != nil {
        return nil, err
    }

    // Create headless config file if provided
    if config.ConfigFile != "" {
        if err := vm.UploadFile(ctx, config.ConfigFile, "/tmp/install-config.yaml"); err != nil {
            return nil, err
        }
    }

    // Build install command
    installCmd := []string{
        "/usr/local/bin/embedded-cluster-smoke-test-staging-app", "install",
        "--license", config.License,
        "--target", "linux",
        "--headless",
        "--yes",
    }

    if config.Scenario == "airgap" {
        installCmd = append(installCmd, "--airgap-bundle", "/tmp/airgap-bundle.tar.gz")
    }

    if config.ConfigFile != "" {
        installCmd = append(installCmd, "--config", "/tmp/install-config.yaml")
    }

    // Run installation command
    stdout, stderr, err := vm.RunCommand(ctx, installCmd, RunOptions{
        Timeout: 30 * time.Minute,
    })
    if err != nil {
        return nil, fmt.Errorf("installation failed: %w\nstdout: %s\nstderr: %s", err, stdout, stderr)
    }

    return &InstallResult{
        Success:         true,
        KubeconfigPath:  "/var/lib/embedded-cluster-smoke-test-staging-app/k0s/pki/admin.conf",
        InstallationLog: stdout,
    }, nil
}

```

#### Validation
```go
// dagger/e2e/validation.go

// validate performs comprehensive installation validation using Kubernetes client
func (m *TestModule) validate(ctx context.Context, scenario string, vm *CMXInstance, result *InstallResult) (*ValidationResult, error) {
    validationResult := &ValidationResult{
        Checks: make(map[string]CheckResult),
    }

    // Run validation checks in order
    checks := []struct {
        name string
        fn   func(context.Context, *CMXInstance, string) error
    }{
        {"kubernetes_cluster_health", m.validateClusterHealth},
        {"installation_crd_status", m.validateInstallationCRD},
        {"application_deployment", m.validateAppDeployment},
        {"admin_console_components", m.validateAdminConsole},
        {"data_directory_configuration", m.validateDataDirectories},
        {"pod_and_job_health", m.validatePodsAndJobs},
    }

    for _, check := range checks {
        err := check.fn(ctx, vm, scenario)
        validationResult.Checks[check.name] = CheckResult{
            Passed: err == nil,
            Error:  err,
        }
        if err != nil {
            validationResult.Success = false
            // Continue running other checks to collect all failures
        }
    }

    validationResult.Success = true
    for _, result := range validationResult.Checks {
        if !result.Passed {
            validationResult.Success = false
            break
        }
    }

    return validationResult, nil
}

// validateClusterHealth validates Kubernetes cluster health
func (m *TestModule) validateClusterHealth(ctx context.Context, vm *CMXInstance, scenario string) error {
    // Check node versions match expected k8s version
    stdout, _, err := vm.RunCommand(ctx, []string{
        "kubectl", "get", "nodes",
        "-o", "jsonpath='{.items[*].status.nodeInfo.kubeletVersion}'",
    })
    if err != nil {
        return fmt.Errorf("failed to get node versions: %w", err)
    }

    expectedVersion := m.expectedK8sVersion
    if !strings.Contains(stdout, expectedVersion) {
        return fmt.Errorf("node version mismatch: got %s, want %s", stdout, expectedVersion)
    }

    // Check node readiness - works for both single-node and multi-node
    stdout, _, err = vm.RunCommand(ctx, []string{
        "kubectl", "get", "nodes",
        "--no-headers",
    })
    if err != nil {
        return fmt.Errorf("failed to get nodes: %w", err)
    }

    // Check that all nodes are Ready and none are NotReady
    if strings.Contains(stdout, "NotReady") {
        return fmt.Errorf("one or more nodes are not ready: %s", stdout)
    }

    lines := strings.Split(strings.TrimSpace(stdout), "\n")
    for _, line := range lines {
        if line == "" {
            continue
        }
        if !strings.Contains(line, "Ready") {
            return fmt.Errorf("node not ready: %s", line)
        }
    }

    return nil
}
```

### External Contracts

No new external APIs or events. Tests will consume existing embedded-cluster APIs and interfaces.

### Usage

The V3 E2E tests are invoked using the Dagger CLI. All tests run through Dagger modules that handle environment provisioning, secret management, and test execution.

#### Running Tests Locally

**Build and release artifacts:**
```bash
dagger call build-and-release
```

**Run all V3 tests (all 4 test runs) with existing artifacts:**
```bash
dagger call run-all-tests
```

**Build, release, and run complete E2E test suite:**
```bash
dagger call build-and-release run-all-tests
```

**Run both modes for online scenario:**
```bash
dagger call run-online-tests
```

**Run both modes for airgap scenario:**
```bash
dagger call run-airgap-tests
```

**Run specific mode and scenario:**
```bash
# Browser-based online installation
dagger call run-browser-based-test --scenario online

# Headless online installation
dagger call run-headless-test --scenario online

# Browser-based airgap installation
dagger call run-browser-based-test --scenario airgap

# Headless airgap installation
dagger call run-headless-test --scenario airgap
```

#### Prerequisites

**1Password Setup:**
Tests require 1Password CLI and service account token:
```bash
export OP_SERVICE_ACCOUNT_TOKEN="your-token-here"
```

**CMX Credentials:**
CMX credentials must be available in 1Password vault `embedded-cluster-e2e` with the following keys:
- `cmx_api_token` - CMX API credentials
- `replicated_api_token` - Replicated API token
- `aws_access_key` - AWS access key for S3
- `aws_secret_key` - AWS secret key for S3

#### Running in CI

GitHub Actions workflow will invoke Dagger commands:
```yaml
- name: Build, Release, and Run V3 E2E Tests
  run: |
    dagger call build-and-release run-all-tests
  env:
    OP_SERVICE_ACCOUNT_TOKEN: ${{ secrets.OP_SERVICE_ACCOUNT_TOKEN }}
```

Or run build and test steps separately:
```yaml
- name: Build and Release
  run: |
    dagger call build-and-release
  env:
    OP_SERVICE_ACCOUNT_TOKEN: ${{ secrets.OP_SERVICE_ACCOUNT_TOKEN }}

- name: Run V3 E2E Tests
  run: |
    dagger call run-all-tests
  env:
    OP_SERVICE_ACCOUNT_TOKEN: ${{ secrets.OP_SERVICE_ACCOUNT_TOKEN }}
```

Tests run in parallel by default using isolated CMX VMs to reduce total runtime.

#### Test Output

Test results include:
- Pass/fail status for each test mode and scenario
- Detailed validation results for each check
- Logs from installation and validation
- Screenshots and traces from Playwright tests (browser-based mode)

No entitlements required as this is test infrastructure only.

## Testing

### Installation Validation

Both browser-based and headless tests will perform comprehensive validation after installation using the Kubernetes client. The validation is based on the proven checks from V2 E2E tests in `e2e/scripts/check-installation-state.sh` and `e2e/scripts/common.sh`.

**Validation Steps (performed by both test modes):**

1. **Kubernetes Cluster Health**
   - Verify all nodes are running the expected k8s version
   - Check kubelet version matches expected version on all nodes
   - Validate node readiness status

2. **Installation CRD Status**
   - Verify Installation resource exists and is in "Installed" state
   - Confirm embedded-cluster operator successfully completed installation

3. **Application Deployment**
   - Wait for application's nginx pods to be Running
   - Verify correct app version is deployed (using kubectl kots or kotsadm API for airgap)
   - Confirm no upgrade artifacts present (kube-state-metrics namespace, "second" app pods)

4. **Admin Console Components**
   - Confirm kotsadm pods are healthy
   - Confirm kotsadm api is health (run "kubectl kots get apps")

5. **Data Directory Configuration**
   - Validate K0s data directory is configured correctly
   - Validate OpenEBS data directory is configured correctly
   - Validate Velero pod volume path is configured correctly
   - Verify all components use expected base directory

6. **Pod and Job Health (comprehensive)**
   - All non-Job pods are in Running/Completed/Succeeded state
   - All Running pods have ready containers
   - All Jobs have completed successfully

**Implementation:**
The V3 tests will reuse existing validation scripts from `e2e/scripts/` where possible, wrapping them for execution via Dagger. New validation helpers will be written in Go as part of the test framework for cases where Go Kubernetes client access is more appropriate.

## Backward Compatibility

- **V2 tests remain completely unchanged**: No modifications to existing V2 test infrastructure
- **Separate V3 framework**: V3 gets its own independent E2E test framework
- **No migration**: V2 tests are NOT being migrated to the V3 framework (different products)
- **V2 deprecation plan**:
  - V2 tests continue to run as-is for V2 installer
  - Once V2 installer is retired, V2 tests will be removed
- **Existing CI workflows**: Continue to function unchanged
- **No changes to embedded-cluster binary or APIs**: Tests are infrastructure-only

## Migrations

No special deployment handling required. The test framework is development tooling only and doesn't affect production systems.

## Trade-offs

### What We're Optimizing For
1. **Simplicity**: Just 2 smoke tests, minimal infrastructure
2. **Developer experience**: Easy local test execution
3. **Portability**: Same behavior locally and in CI
4. **Speed**: Fast feedback cycles for basic V3 validation
5. **Isolation**: Tests run concurrently using isolated CMX VMs to reduce total runtime

### What We're Trading Off
1. **Limited E2E coverage**: Only testing external dependencies, not comprehensive (acceptable because unit and integration tests provide comprehensive coverage)
2. **Initial setup**: Dagger and CMX add new tools to learn
3. **1Password dependency**: New external service dependency

### Key Architectural Decisions

1. **Use Dagger for test orchestration**: All test infrastructure will be built using Dagger for portability
   - **Rationale**: Dagger provides consistent execution locally and in CI, with built-in dependency management
   - **Benefit**: Same tests run identically on developer machines and CI, no environment-specific setup
   - **Approach**: Build Dagger modules for CMX provisioning, test execution, and validation
   - **Portability**: Tests work on macOS, Linux, and in GitHub Actions without modification

2. **Use 1Password for secret management**: All secrets will be managed through 1Password
   - **Rationale**: Centralized secret management eliminates scattered secrets across GitHub Actions, local envs, and developer machines
   - **Benefit**: Team members can access secrets via 1Password vault, rotating secrets is simple, no secrets in code
   - **Approach**: Use 1Password Dagger module to fetch secrets at runtime
   - **Secrets include**: Replicated API tokens, CMX credentials, AWS credentials

3. **Wrap existing build scripts**: We will use Dagger to wrap `scripts/build-and-release.sh` and related scripts rather than rewriting them in Go
   - **Rationale**: These scripts are battle-tested, handle complex orchestration, and work reliably
   - **Benefit**: Reduces implementation risk, faster time to value, maintains existing CI compatibility
   - **Approach**: Create Dagger containers that execute existing bash scripts with proper environment variables
   - **Scripts involved**: build-and-release.sh, ci-build-deps.sh, ci-build-bin.sh, ci-embed-release.sh, ci-upload-binaries.sh, ci-release-app.sh

4. **Use CMX exclusively for V3 tests**: All V3 tests will use CMX, creating a clean break from the V2 hybrid approach
   - **Current problem**: V2 tests use hybrid Docker/LXD/CMX approach creating inconsistency
   - **Decision**: V3 test framework uses CMX exclusively for ALL test scenarios
   - **Rationale**:
     - **Portability**: CMX works across platforms; LXD is Linux-specific
     - **Advanced scenarios**: CMX supports airgap testing natively (hopefully proxy testing in the future)
     - **Consistency**: Single provisioning approach for all tests, no special cases
     - **Already proven**: CMX is already used successfully in the codebase
   - **V2 tests**: Explicitly NOT being modified
     - V2 tests keep their existing Docker/LXD/CMX hybrid approach
     - No migration effort from V2 to V3 framework
     - V2 tests will be deprecated once V3 upgrade path exists
     - V2 tests eventually removed when V2 installer is retired
   - **V3 test scenarios** (2 scenarios × 2 modes = 4 tests, all CMX):
     1. Online installation - browser-based (Playwright)
     2. Online installation - headless (CLI)
     3. Airgap installation - browser-based (Playwright)
     4. Airgap installation - headless (CLI)

## Alternative Solutions Considered

### 1. Rewrite Build Scripts in Go
- **Why rejected**:
  - Existing bash scripts are battle-tested and reliable
  - Rewriting would introduce risk and require extensive validation
  - No performance or maintainability benefit
  - Wrapping existing scripts in Dagger containers is faster and safer
  - Maintains compatibility with current CI workflows during transition

### 2. Continue Hybrid Docker/LXD/CMX Approach for V3
- **Why rejected**:
  - LXD is not portable (Linux-specific, requires host configuration)
  - Maintaining three different provisioning systems increases complexity
  - Inconsistent test environments make debugging harder
  - CMX already supports most scenarios we need (including airgap)
  - Standardizing on CMX-only for V3 reduces maintenance burden and improves portability
  - Clean break from V2's technical debt

### 3. Migrate V2 Tests to V3 Framework
- **Why rejected**:
  - V2 tests are for V2 installer, not V3 - they test a different product
  - No value in migrating tests for a different version of the product
  - Risk of breaking existing V2 test coverage during migration
  - V2 tests will be removed when V2 installer is retired
  - V3 needs its own E2E tests focused on external dependencies

## Research

### Prior Art in Codebase
- Research document: [V3 E2E Tests Research](./v3_e2e_tests_research.md)
- Existing Dagger modules: `/dagger/` directory with Chainguard, operator builds
- Playwright tests: `/e2e/playwright/` with UI test coverage
- Current E2E tests: `/e2e/` directory with Docker/LXD-based tests

### External References
- [Dagger Documentation](https://docs.dagger.io/)
- [1Password Dagger Module](https://github.com/dagger/dagger/tree/main/modules/onepassword)
- [Playwright Test Runner](https://playwright.dev/docs/test-runners)
- [TestContainers](https://www.testcontainers.org/) - Similar approach for test isolation

### Prototypes
No prototypes were built for this proposal, but the patterns are validated by:
- Existing Dagger usage in the codebase
- Industry adoption of Dagger for CI/CD portability
- Success of similar approaches in other Replicated projects

## Checkpoints (PR Plan)

### PR 1: Foundation and Secret Management
- Create base E2E test structure in `dagger/e2e/`
- Add 1Password Dagger module
- Create CMX VM provisioning module in Dagger
- Add secret fetching logic
- Update documentation

### PR 2: Build Module and Artifact Management
- Create Dagger wrapper for `scripts/build-and-release.sh`
- Wrap existing CI scripts (ci-build-deps.sh, ci-build-bin.sh, etc.) in Dagger containers
- Create artifact versioning system
- Validate that wrapped scripts produce identical outputs to current CI

### PR 3: Headless Installation E2E Tests (Both Scenarios)
- Implement online installation E2E test - **headless** (CLI, no Playwright)
- Implement airgap installation E2E test - **headless** (CLI, no Playwright)
- Add Kube client validation helpers (used by both modes):
  - Kubernetes cluster health checks
  - Installation CRD status validation
  - Application deployment verification
  - Admin console component health checks
  - Data directory configuration validation
  - Comprehensive pod and job health validation

### PR 4: CI Integration + Documentation
- Add V3 E2E tests to GitHub Actions workflows
- Add Dagger commands to CI (runs alongside V2 tests)
- Document CMX setup for local development
- Document local test execution
- Add troubleshooting guide
- Update README files

### PR 5: Browser-Based Installation E2E Tests (Both Scenarios)
- Wrap Playwright tests with Dagger for browser-based mode
- Implement online installation E2E test - **browser-based** (Playwright)
- Implement airgap installation E2E test - **browser-based** (Playwright)

Each PR will be independently reviewable and testable, with all 4 test runs (2 scenarios × 2 modes) functional and running in CI after PR 5.