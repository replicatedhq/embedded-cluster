# Headless Installation for Embedded Cluster V3

## TL;DR (solution in one paragraph)

We're implementing headless installation support for embedded cluster v3 that allows users to install applications without UI interaction by passing kots.io/v1beta1.ConfigValues and installation parameters via CLI flags. The implementation will run the v3 API server in-process within the CLI (not as a separate server process) and use it as an abstraction layer to maintain consistency with browser-based installations. The CLI will programmatically authenticate, automatically progress through the state machine transitions, and provide clear CLI feedback for all operations. This enables CI/CD automation, testing pipelines, and unattended installations while maintaining a single code path for installation logic.

## The problem

Currently, embedded cluster v3 requires users to interact with the Manager UI to complete installations. This prevents automation scenarios like CI/CD pipelines, automated testing, and unattended deployments. While v2 has some headless support, it doesn't use the v3 API and creates a divergent code path that's harder to maintain. Users need a way to:

- Install applications without manual UI interaction
- Provide kots.io/v1beta1.ConfigValues programmatically
- Run installations in CI/CD pipelines
- Automate testing workflows
- Deploy at scale without manual intervention

Evidence of need:

- Shortcut story SC-128914 explicitly requests this feature
- Testing teams need headless installs for automation
- Current v2 headless implementation has been problematic (confusing, issues to rectify)
- Enterprise customers require unattended installation capabilities

## Prototype / design

### CLI Interface Design

```bash
# Headless install with kots.io/v1beta1.ConfigValues
embedded-cluster install \
  --target linux \
  --license ./license.yaml \
  --config-values ./config.yaml \
  --admin-console-password mypassword \
  --headless \
  --yes

# Headless install with preflight bypass
embedded-cluster install \
  --target linux \
  --license ./license.yaml \
  --config-values ./config.yaml \
  --admin-console-password mypassword \
  --headless \
  --ignore-host-preflights \
  --ignore-app-preflights \
  --yes
```

### Architecture Flow

```
CLI (--headless flag)
    ↓
Validate required flags (--config-values, --admin-console-password)
Load kots.io/v1beta1.ConfigValues from file
    ↓
Start API server in-process (same process as CLI, not separate daemon)
    ↓
Generate auth token programmatically
    ↓
Create headless/install.Orchestrator (wraps in-process API client)
    ↓
RunHeadlessInstall:
  - Validate & patch config values via API
  - Configure installation settings via API
  - If validation fails, return error immediately
    ↓
State Machine Auto-Progression:
  - Run Host Preflights (Linux only)
  - Install Infrastructure
  - Process Airgap (if needed)
  - Run App Preflights
  - Install Application
    ↓
RunHeadlessInstall completes
    ↓
Report success/failure
    ↓
API server shuts down when CLI exits
```

### Key Components

1. **In-Process API Server**: The v3 API server runs within the CLI process itself, not as a separate daemon. This provides an abstraction layer that maintains consistency with browser-based installations while keeping everything in a single process. The API server conditionally skips web server initialization when `Headless` flag is set, running only the API endpoints needed for headless operation.

2. **headless/install.Orchestrator Interface**: Defines the contract for headless installation operations with a single method `RunHeadlessInstall()`. The concrete `orchestrator` implementation wraps `api/client.Client` to manage automatic state progression and progress reporting via in-process HTTP calls to the API server.

3. **HeadlessInstallOptions**: Configuration struct containing all options for a headless installation including config values (`apitypes.AppConfigValues`), installation settings, preflight bypass flags, and airgap bundle path.

4. **api/client.Client**: Existing low-level HTTP client (already implemented at `api/client/client.go`) that provides methods to interact with v3 API endpoints via standard HTTP calls (localhost).

5. **spinner.MessageWriter**: Existing progress reporter from `pkg/spinner` that outputs progress to CLI with checkmarks and spinners, matching the existing installation output format.

### Output Format

The headless installation will match the existing installation output format as closely as possible:

```bash
$ ./my-app install --target linux --license license.yaml --config-values config.yaml --admin-console-password password --yes --headless

✔  Initialization complete
✔  Host preflights passed
✔  Node is ready
✔  Storage is ready
✔  Runtime Operator is ready
✔  Disaster Recovery is ready
✔  Admin Console is ready
✔  Installing additional components (2/2)
✔  Installation complete

Installation completed successfully
```

**With bypassed preflight failures:**

```bash
$ ./my-app install --target linux --license license.yaml --config-values config.yaml --admin-console-password password --yes --headless --ignore-host-preflights

✔  Initialization complete
✗  Host preflights completed with failures

⚠ Warning: Host preflight checks completed with failures

  [ERROR] Insufficient disk space: 10GB available, 50GB required
  [WARN] CPU count below recommended: 2 cores, 4 recommended

Installation will continue, but the system may not meet requirements (failures bypassed with flag).

✔  Node is ready
✔  Storage is ready
✔  Runtime Operator is ready
✔  Disaster Recovery is ready
✔  Admin Console is ready
✔  Installing additional components (2/2)
✔  Installation complete

Installation completed successfully
```

## New Subagents / Commands

No new subagents or commands will be created. This feature extends the existing `install` command with new flags.

## Goals

- Enable headless installations for CI/CD automation and testing
- Use explicit `--headless` flag for clear user intent
- Leverage existing v3 API infrastructure for consistency
- Provide clear CLI progress feedback and error reporting
- Support all installation scenarios (online, airgap, preflight bypass)
- Validate config values and installation config at the start of the headless install flow to fail fast before installation begins

## Non-Goals

- **Installation resumption**: If a headless installation fails, it cannot be resumed. Users must run `reset` and start over. This is consistent with current v3 behavior and simplifies the implementation by avoiding complex state recovery logic.
- **Kubernetes target support**: Initial implementation will only support Linux target (`--target linux`). Kubernetes target support for headless installs may be added in a future iteration.
- **Multi-node cluster setup**: Initial implementation will only support single-node installations. Multi-node cluster setup (adding controller or worker nodes via headless `join` command) may be added in a future iteration.
- **Remote headless installs**: The API server will run in-process within the CLI, not as a separate daemon. This means the CLI must run on the target installation host. Remote installations where the CLI runs on a different machine from the API server are not supported. This simplifies the architecture and avoids the complexity of managing a separate API server process, network configuration, and remote authentication.
- **Alternative output formats**: Headless installation output will be human-readable text only. Machine-readable formats (JSON, YAML, etc.) are not supported in the initial implementation. The focus is on providing clear, actionable feedback for humans monitoring installations. Future iterations may add structured output formats if needed for advanced automation scenarios.
- **Application links display**: The headless installation will not compute or display application links at the end of installation. The installation will complete with a generic success message. Computing and displaying application links will be added in a future iteration.
- Backwards compatibility with v2 headless install semantics (v2 continues to work separately)

## Database

**No database changes required.**

The headless installation uses the same underlying storage mechanisms as the UI-based installation through the existing API and state management infrastructure.

## Implementation plan

**Note on Architecture**: The API server will be started in-process within the CLI when `--headless` flag is used. The server runs on localhost only and shuts down when the CLI exits. All API interactions happen via HTTP calls to localhost within the same process.

### Files/Services to Touch

1. **CLI Layer**:
   - `cmd/installer/cli/install.go`: Add `--headless` flag, integrate headless mode logic, handle config values loading
   - `cmd/installer/cli/install_v3.go`: Add `runV3InstallHeadless()` main flow, `buildOrchestrator()` helper function, and `buildHeadlessInstallOptions()` helper function (separated for unit testing)
   - `cmd/installer/cli/install_config.go`: Add `validateHeadlessInstallFlags()` for flag validation (required flags, target validation)
   - `cmd/installer/cli/api.go`: Refactor `startAPI()` to return error channel for better lifecycle management, add `Headless` flag to `apiOptions` to conditionally skip web server initialization
   - `cmd/installer/cli/headless/install/orchestrator.go`: NEW - Define `Orchestrator` interface and `orchestrator` implementation with `RunHeadlessInstall()` method, define `HeadlessInstallOptions` struct
   - `cmd/installer/cli/headless/install/mock_orchestrator.go`: NEW - Mock implementation for testing
   - `cmd/installer/cli/headless/install/progress.go`: NEW - Progress reporting and error formatting (future PR)
   - `cmd/installer/cli/headless/install/client.go`: NEW - API client wrapper if needed (future PR)

2. **API Layer** (runs in-process):
   - `api/controllers/auth/controller.go`: Add programmatic token generation for in-process authentication (future PR)
   - `api/controllers/app/controller.go`: Remove early config value validation from `NewAppController`. Defer validation for browser-based and headless installs until the API calls are made in the browser and corresponding `RunHeadlessInstall` function to make install paths consistent.
   - `api/handlers.go`: Ensure handlers work with headless orchestrator, return structured errors (future PR)

3. **Test Infrastructure**:
   - `cmd/installer/cli/install_config_test.go`: Add unit tests for headless flag validation
   - `cmd/installer/cli/install_v3_test.go`: NEW - Unit tests for `buildOrchestrator()` and `buildHeadlessInstallOptions()` functions
   - `tests/dryrun/v3_install_test.go`: NEW - Dryrun integration tests for headless install
   - `tests/dryrun/assets/kotskinds-*.yaml`: NEW - Test assets for KOTS kinds
   - `pkg-new/replicatedapi/client.go`: Add `ClientFactory` pattern for dependency injection
   - `pkg/dryrun/replicatedapi.go`: NEW - Mock Replicated API client for testing

4. **State Management** (future PR):
   - `api/internal/statemachine/auto_progress.go`: NEW - Auto-progression logic with failure detection (if needed)

### Pseudo Code

```go
// cmd/installer/cli/headless/install/orchestrator.go

// HeadlessInstallOptions contains the configuration options for a headless installation
type HeadlessInstallOptions struct {
    // ConfigValues are the application config values to use for installation
    ConfigValues apitypes.AppConfigValues

    // LinuxInstallationConfig contains the installation settings for the Linux target
    LinuxInstallationConfig apitypes.LinuxInstallationConfig

    // IgnoreHostPreflights indicates whether to bypass host preflight check failures
    IgnoreHostPreflights bool

    // IgnoreAppPreflights indicates whether to bypass app preflight check failures
    IgnoreAppPreflights bool

    // AirgapBundle is the path to the airgap bundle file (empty string for online installs)
    AirgapBundle string
}

// Orchestrator defines the interface for headless installation operations.
// It orchestrates the installation process by interacting with the v3 API server
// running in-process via HTTP calls to localhost.
type Orchestrator interface {
    // RunHeadlessInstall executes a complete headless installation workflow.
    // It performs the following steps in order:
    //   1. Configure application with config values
    //   2. Configure installation settings
    //   3. Run host preflights (with optional bypass)
    //   4. Setup infrastructure (POINT OF NO RETURN)
    //   5. Process airgap bundle (if provided)
    //   6. Run app preflights (with optional bypass)
    //   7. Install application
    //
    // The installation cannot be resumed if it fails after infrastructure setup begins (step 4).
    // Any failure after that point requires running 'embedded-cluster reset' and retrying.
    //
    // Returns:
    //   - resetNeeded: true if the failure requires running 'embedded-cluster reset' before retrying
    //   - err: the error that occurred, or nil on success
    RunHeadlessInstall(ctx context.Context, opts HeadlessInstallOptions) (resetNeeded bool, err error)
}

// orchestrator is the concrete implementation of the Orchestrator interface
type orchestrator struct {
    apiClient      client.Client
    target         apitypes.InstallTarget // "linux" or "kubernetes"
    progressWriter spinner.WriteFn        // Output writer for progress messages
    logger         logrus.FieldLogger     // Logger for detailed logging
}

func NewOrchestrator(apiClient client.Client, password string, target string, opts ...OrchestratorOption) (Orchestrator, error) {
    // We do not yet support the "kubernetes" target
    if target != apitypes.InstallTargetLinux {
        return nil, fmt.Errorf("%s target not supported", target)
    }

    // Authenticate and set token
    if err := apiClient.Authenticate(password); err != nil {
        return nil, fmt.Errorf("authentication failed: %w", err)
    }

    o := &orchestrator{
        apiClient:      apiClient,
        target:         target,
        progressWriter: fmt.Printf,
        logger:         logrus.StandardLogger(),
    }

    for _, opt := range opts {
        opt(o)
    }

    return o, nil
}

// OrchestratorOption is a functional option for configuring the orchestrator
type OrchestratorOption func(*orchestrator)

// WithProgressWriter sets a custom progress writer for the orchestrator
func WithProgressWriter(writer spinner.WriteFn) OrchestratorOption {
    return func(o *orchestrator) {
        o.progressWriter = writer
    }
}

func (o *orchestrator) RunHeadlessInstall(ctx context.Context, opts HeadlessInstallOptions) (bool, error) {
    // Configure application with config values
    if err := o.configureApplication(ctx, opts); err != nil {
        return false, err // Can retry without reset
    }

    // Configure installation
    if err := o.configureInstallation(ctx, opts); err != nil {
        return false, err // Can retry without reset
    }

    // Run host preflights (allow bypass when --ignore-host-preflights flag is set)
    if err := o.runHostPreflights(ctx, opts.IgnoreHostPreflights); err != nil {
        return false, err // Can retry without reset
    }

    // Setup infrastructure (POINT OF NO RETURN)
    // After this point, any failure requires running 'embedded-cluster reset'
    if err := o.setupInfrastructure(ctx, opts.IgnoreHostPreflights); err != nil {
        return true, err // Reset required
    }

    // Process airgap if needed
    if opts.AirgapBundle != "" {
        if err := o.processAirgap(ctx); err != nil {
            return true, err // Reset required
        }
    }

    // Run app preflights (allow bypass when --ignore-app-preflights flag is set)
    if err := o.runAppPreflights(ctx, opts.IgnoreAppPreflights); err != nil {
        return true, err // Reset required
    }

    // Install application
    if err := o.installApp(ctx, opts.IgnoreAppPreflights); err != nil {
        return true, err // Reset required
    }

    return false, nil // Success
}

// formatAPIError formats an APIError for display to the user
func formatAPIError(apiErr *types.APIError) string {
    if apiErr == nil {
        return ""
    }

    var buf strings.Builder

    // Write the main error message if present
    if apiErr.Message != "" {
        buf.WriteString(apiErr.Message)
    }

    // Write field errors
    if len(apiErr.Errors) > 0 {
        if buf.Len() > 0 {
            buf.WriteString(":\n")
        }
        for _, fieldErr := range apiErr.Errors {
            if fieldErr.Field != "" {
                buf.WriteString(fmt.Sprintf("  - Field '%s': %s\n", fieldErr.Field, fieldErr.Message))
            } else {
                buf.WriteString(fmt.Sprintf("  - %s\n", fieldErr.Message))
            }
        }
    }

    result := buf.String()
    // Remove trailing newline
    if strings.HasSuffix(result, "\n") {
        result = result[:len(result)-1]
    }
    return result
}

// Example method showing how orchestrator wraps api/client.Client
func (o *orchestrator) configureApplication(ctx context.Context, opts HeadlessInstallOptions) error {
    o.logger.Debug("Starting application configuration")

    loading := spinner.Start(spinner.WithWriter(o.progressWriter))
    loading.Infof("Configuring application...")

    // Use the wrapped api/client.Client to patch config values
    _, err := o.apiClient.PatchLinuxInstallAppConfigValues(opts.ConfigValues)
    if err != nil {
        loading.ErrorClosef("Application configuration failed")

        // Check if it's an APIError with field details
        var apiErr *types.APIError
        if errors.As(err, &apiErr) && len(apiErr.Errors) > 0 {
            // Format and display the structured error
            formattedErr := formatAPIError(apiErr)
            return fmt.Errorf("application configuration validation failed:\n%s", formattedErr)
        }

        return fmt.Errorf("patch app config values: %w", err)
    }

    loading.Closef("Application configuration complete")
    o.logger.Debug("Application configuration complete")
    return nil
}

// Another example showing state monitoring
func (o *orchestrator) setupInfrastructure(ctx context.Context, ignoreHostPreflights bool) error {
    o.logger.Debug("Starting infrastructure setup")

    loading := spinner.Start(spinner.WithWriter(o.progressWriter))
    loading.Infof("Setting up infrastructure...")

    // Initiate infra setup using api/client.Client
    infra, err := o.apiClient.SetupLinuxInfra(ignoreHostPreflights)
    if err != nil {
        loading.ErrorClosef("Infrastructure setup failed: %v", err)
        return fmt.Errorf("setup linux infra: %w", err)
    }

    getStatus := func() (types.State, string, error) {
        infra, err = o.apiClient.GetLinuxInfraStatus()
        if err != nil {
            return types.State{}, "", err
        }
        return infra.Status.State, infra.Status.Description, nil
    }

    // Poll for completion using api/client.Client
    err = pollUntilComplete(ctx, getStatus)
    if err != nil {
        loading.ErrorClosef("Infrastructure setup failed: %v", err)
        return fmt.Errorf("poll until complete: %w", err)
    }

    loading.Closef("Infrastructure setup complete")
    o.logger.Debug("Infrastructure setup complete")
    return nil
}

```

### CLI Integration

```go
// cmd/installer/cli/install.go

func runManagerExperienceInstall(...) (finalErr error) {
    // ... existing setup code ...

    if flags.headless {
        return runV3InstallHeadless(ctx, cancel, flags, installCfg, apiOpts)
    }

    // ... existing UI flow ...
}
```

```go
// cmd/installer/cli/install_v3.go

func runV3InstallHeadless(
    ctx context.Context,
    cancel context.CancelFunc,
    flags installFlags,
    installCfg *installConfig,
    apiOpts apiOptions,
) error {
    // Setup signal handler
    signalHandler(ctx, cancel, func(ctx context.Context, sig os.Signal) {
        apiOpts.MetricsReporter.ReportSignalAborted(ctx, sig)
    })

    // Build orchestrator
    orchestrator, err := buildOrchestrator(installCfg, apiOpts)
    if err != nil {
        return fmt.Errorf("failed to build orchestrator: %w", err)
    }

    // Build install options
    opts := buildHeadlessInstallOptions(flags, apiOpts)

    resetNeeded, err := orchestrator.RunHeadlessInstall(ctx, opts)
    if err != nil {
        if errors.Is(err, terminal.InterruptErr) {
            apiOpts.MetricsReporter.ReportSignalAborted(ctx, syscall.SIGINT)
        } else {
            apiOpts.MetricsReporter.ReportInstallationFailed(ctx, err)
        }

        // Print error and recovery instructions
        logrus.Errorf("\nError: %v\n", err)

        if resetNeeded {
            logrus.Info("To collect diagnostic information, run: embedded-cluster support-bundle")
            logrus.Info("To retry installation, run: embedded-cluster reset and wait for server reboot")
        } else {
            logrus.Info("Please correct the above issues and retry")
        }

        return NewErrorNothingElseToAdd(err)
    }

    // Display success message
    logrus.Info("\nInstallation completed successfully")

    apiOpts.MetricsReporter.ReportInstallationSucceeded(ctx)
    return nil
}

// Hop: buildOrchestrator creates an orchestrator from CLI inputs.
func buildOrchestrator(
    installCfg *installConfig,
    apiOpts apiOptions,
) (install.Orchestrator, error) {
    // Construct API URL from manager port
    apiURL := fmt.Sprintf("https://localhost:%d", installCfg.managerPort)

    // We do not yet support the "kubernetes" target
    if target != apitypes.InstallTargetLinux {
        return nil, fmt.Errorf("%s target not supported", target)
    }

    // Create HTTP client with InsecureSkipVerify for localhost
    // Since the API server is in-process and on localhost only, certificate
    // validation is not critical for this use case
    httpClient := &http.Client{
        Timeout: 30 * time.Second,
        Transport: &http.Transport{
            Proxy: nil,  // No proxy for localhost
            TLSClientConfig: &tls.Config{
                InsecureSkipVerify: true,  // Acceptable for localhost in-process API
            },
        },
    }

    // Create API client
    apiClient := client.New(
        apiURL,  // e.g., "https://localhost:30000"
        client.WithHTTPClient(httpClient),
    )

    // Create orchestrator
    orchestrator, err := install.NewOrchestrator(
        apiClient,
        apiOpts.Password,
        apiOpts.InstallTarget,
    )
    if err != nil {
        return nil, fmt.Errorf("failed to create orchestrator: %w", err)
    }

    return orchestrator, nil
}

// Hop: buildHeadlessInstallOptions creates HeadlessInstallOptions from CLI inputs.
func buildHeadlessInstallOptions(
    flags installFlags,
    apiOpts apiOptions,
) install.HeadlessInstallOptions {
    // Build Linux installation config from flags
    linuxInstallationConfig := apitypes.LinuxInstallationConfig{
        AdminConsolePort:        flags.adminConsolePort,
        DataDirectory:           flags.dataDir,
        LocalArtifactMirrorPort: flags.localArtifactMirrorPort,
        HTTPProxy:               flags.httpProxy,
        HTTPSProxy:              flags.httpsProxy,
        NoProxy:                 flags.noProxy,
        NetworkInterface:        flags.networkInterface,
        PodCIDR:                 flags.podCIDR,
        ServiceCIDR:             flags.serviceCIDR,
        GlobalCIDR:              flags.globalCIDR,
    }

    return install.HeadlessInstallOptions{
        ConfigValues:            apiOpts.ConfigValues,
        LinuxInstallationConfig: linuxInstallationConfig,
        IgnoreHostPreflights:    flags.ignoreHostPreflights,
        IgnoreAppPreflights:     flags.ignoreAppPreflights,
        AirgapBundle:            flags.airgapBundle,
    }
}

```

## Implementation Details

This section provides concrete specifications needed for implementation.

### API Endpoint Mapping

Complete mapping of orchestrator steps to API endpoints (from `api/routes.go:45-72`):

| Called By (Orchestrator Method) | HTTP Method | Endpoint | Request Body | Response Type | Polling? |
|---------------------------------|-------------|----------|--------------|---------------|----------|
| `NewOrchestrator()` | POST | `/api/auth/login` | `{"password": string}` | `{"token": string}` | No |
| *(Not used in headless flow)* | GET | `/api/linux/install/installation/config` | - | `LinuxInstallationConfigResponse` | No |
| `configureInstallation()` | POST | `/api/linux/install/installation/configure` | `LinuxInstallationConfig` | `Status` | No |
| `configureInstallation()` | GET | `/api/linux/install/installation/status` | - | `Status` | Yes |
| `configureApplication()` | PATCH | `/api/linux/install/app/config/values` | `AppConfigValues` | `AppConfigValues` | No |
| *(Not used in headless flow)* | GET | `/api/linux/install/app/config/values` | - | `AppConfigValues` | No |
| *(Not used in headless flow)* | POST | `/api/linux/install/app/config/template` | `AppConfigValues` | `AppConfig` | No |
| `runHostPreflights()` | POST | `/api/linux/install/host-preflights/run` | `{"ignoreFailures": bool}` | Status response | No |
| `runHostPreflights()` | GET | `/api/linux/install/host-preflights/status` | - | `InstallHostPreflightsStatusResponse` | Yes |
| `setupInfrastructure()` | POST | `/api/linux/install/infra/setup` | `{"ignoreHostPreflights": bool}` | `Infra` | No |
| `setupInfrastructure()` | GET | `/api/linux/install/infra/status` | - | `Infra` | Yes |
| `processAirgap()` | POST | `/api/linux/install/airgap/process` | - | Status response | No |
| `processAirgap()` | GET | `/api/linux/install/airgap/status` | - | `Airgap` | Yes |
| `runAppPreflights()` | POST | `/api/linux/install/app-preflights/run` | - | Status response | No |
| `runAppPreflights()` | GET | `/api/linux/install/app-preflights/status` | - | `InstallAppPreflightsStatusResponse` | Yes |
| `installApp()` | POST | `/api/linux/install/app/install` | - | `AppInstall` | No |
| `installApp()` | GET | `/api/linux/install/app/status` | - | `AppInstall` | Yes |

**Notes**:

- "Polling?" indicates whether the operation is asynchronous and requires polling the status endpoint until completion.
- Methods in `RunHeadlessInstall()` call sequence: `configureApplication()` → `configureInstallation()` → `runHostPreflights()` → `setupInfrastructure()` → `processAirgap()` (if airgap) → `runAppPreflights()` → `installApp()`

### API Client Interface

The `api/client.Client` interface already exists (see `api/client/client.go`) with most methods implemented. The orchestrator will use this interface to interact with the API.

**Existing methods needed:**

```go
type Client interface {
    // Authentication
    Authenticate(password string) error

    // Installation configuration
    GetLinuxInstallationConfig() (types.LinuxInstallationConfigResponse, error)
    ConfigureLinuxInstallation(config types.LinuxInstallationConfig) (types.Status, error)
    GetLinuxInstallationStatus() (types.Status, error)

    // App configuration
    GetLinuxInstallAppConfigValues() (types.AppConfigValues, error)
    PatchLinuxInstallAppConfigValues(types.AppConfigValues) (types.AppConfigValues, error)
    TemplateLinuxInstallAppConfig(values types.AppConfigValues) (types.AppConfig, error)

    // Infrastructure
    SetupLinuxInfra(ignoreHostPreflights bool) (types.Infra, error)
    GetLinuxInfraStatus() (types.Infra, error)

    // App preflights
    RunLinuxInstallAppPreflights() (types.InstallAppPreflightsStatusResponse, error)
    GetLinuxInstallAppPreflightsStatus() (types.InstallAppPreflightsStatusResponse, error)

    // App installation
    InstallLinuxApp() (types.AppInstall, error)
    GetLinuxAppInstallStatus() (types.AppInstall, error)
}
```

**Methods that need to be added** (future PR):

```go
// Host preflights (missing from current interface)
RunLinuxInstallHostPreflights(ignoreFailures bool) (types.InstallHostPreflightsStatusResponse, error)
GetLinuxInstallHostPreflightsStatus() (types.InstallHostPreflightsStatusResponse, error)

// Airgap processing (missing from current interface)
ProcessLinuxAirgap() (types.Airgap, error)
GetLinuxAirgapStatus() (types.Airgap, error)
```

### State Machine & Polling Logic

Asynchronous operations require polling status endpoints until reaching a terminal state. Each operation has specific status values to check:

**Polling Pattern:**

```go
func pollUntilComplete(ctx context.Context, getStatus func() (types.State, string, error)) error {
    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-ticker.C:
            // Retry getStatus() up to 3 times on error
            var state types.State
            var message string
            var err error

            for attempt := 1; attempt <= 3; attempt++ {
                state, message, err = getStatus()
                if err == nil {
                    break
                }

                // If not the last attempt, wait a bit before retrying
                if attempt < 3 {
                    time.Sleep(time.Second)
                }
            }

            // If still erroring after 3 attempts, fail
            if err != nil {
                return fmt.Errorf("get status failed after 3 attempts: %w", err)
            }

            // Check for terminal states (types.State enum values)
            switch state {
            case types.StateSucceeded: // SUCCESS
                return nil
            case types.StateFailed: // FAILURE
                if message != "" {
                    return fmt.Errorf("%s", message)
                }
                return fmt.Errorf("operation failed")
            case types.StatePending, types.StateRunning: // KEEP POLLING
                continue
            default:
                return fmt.Errorf("unknown state: %s", state)
            }
        }
    }
}
```

**Status Values by Operation:**

All operations use the same `types.Status` struct with `types.State` enum:

```go
// api/types/status.go
type Status struct {
    State       State     `json:"state"`
    Description string    `json:"description"`
    LastUpdated time.Time `json:"lastUpdated"`
}

type State string

const (
    StatePending   State = "Pending"   // Not started
    StateRunning   State = "Running"   // In progress
    StateSucceeded State = "Succeeded" // Completed successfully
    StateFailed    State = "Failed"    // Operation failed
)
```

**Response Types:**

1. **Host Preflights**: `types.InstallHostPreflightsStatusResponse.Status`
2. **Infrastructure Setup**: `types.Infra.Status`
3. **Airgap Processing**: `types.Airgap.Status`
4. **App Preflights**: `types.InstallAppPreflightsStatusResponse.Status`
5. **App Installation**: `types.AppInstall.Status`

All use the same State values: `Pending`, `Running`, `Succeeded`, `Failed`

**Concrete Polling Example:**

```go
func (o *orchestrator) setupInfrastructure(ctx context.Context, ignoreHostPreflights bool) error {
    o.logger.Debug("Starting infrastructure setup")

    loading := spinner.Start(spinner.WithWriter(o.progressWriter))
    loading.Infof("Setting up infrastructure...")

    // Trigger setup (non-blocking)
    infra, err := o.apiClient.SetupLinuxInfra(ignoreHostPreflights)
    if err != nil {
        loading.ErrorClosef("Infrastructure setup failed: %v", err)
        return fmt.Errorf("setup linux infra: %w", err)
    }

    getStatus := func() (types.State, string, error) {
        infra, err = o.apiClient.GetLinuxInfraStatus()
        if err != nil {
            return types.State{}, "", err
        }
        return infra.Status.State, infra.Status.Description, nil
    }

    // Poll for completion using api/client.Client
    err = pollUntilComplete(ctx, getStatus)
    if err != nil {
        loading.ErrorClosef("Infrastructure setup failed: %v", err)
        return fmt.Errorf("poll until complete: %w", err)
    }

    loading.Closef("Infrastructure setup complete")
    o.logger.Debug("Infrastructure setup complete")
    return nil
}
```

**Polling Timeouts:**

- Host preflights: 5 minutes
- Infrastructure setup: 30 minutes
- Airgap processing: 60 minutes (large images)
- App preflights: 10 minutes
- App installation: 30 minutes

### Authentication Flow

The CLI authenticates with the in-process API server using the admin console password:

**Authentication Flow:**
- Endpoint: `POST /api/auth/login`
- Request body: `{"password": "admin-console-password"}`
- Response body: `{"token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."}`
- All subsequent requests include the token as a Bearer token: `Authorization: Bearer <token>`

The `api/client.Client` interface already provides an `Authenticate(password string) error` method that handles this flow. No new authentication mechanism is needed for headless mode.

### Error Response Format

The API returns structured errors as `types.APIError`:

```go
type APIError struct {
    StatusCode int         `json:"statusCode,omitempty"`
    Message    string      `json:"message"`              // Main error message
    Field      string      `json:"field,omitempty"`      // Field name (for field-level errors)
    Errors     []*APIError `json:"errors,omitempty"`     // Nested field errors
}
```

**Example error responses:**

1. **Config validation failure (with field errors):**

```json
{
  "statusCode": 400,
  "message": "config validation failed",
  "errors": [
    {
      "message": "required field missing",
      "field": "database_host"
    },
    {
      "message": "value \"10\" exceeds maximum allowed value 5",
      "field": "replica_count"
    },
    {
      "message": "validation rule failed: SSL requires cert_path to be set",
      "field": "enable_ssl"
    }
  ]
}
```

2. **Network configuration conflict:**

```json
{
  "statusCode": 400,
  "message": "installation settings validation failed",
  "errors": [
    {
      "message": "Pod CIDR 10.96.0.0/12 overlaps with service CIDR 10.96.0.0/16"
    }
  ]
}
```

3. **Infrastructure setup failure:**

```json
{
  "statusCode": 500,
  "message": "infrastructure setup failed",
  "errors": [
    {
      "message": "K0s failed to start: context deadline exceeded"
    }
  ]
}
```

**Error Handling Pattern:**

The orchestrator uses a generic `formatAPIError()` helper function to format structured errors for display:

```go
// formatAPIError formats an APIError for display to the user
func formatAPIError(apiErr *types.APIError) string {
    if apiErr == nil {
        return ""
    }

    var buf strings.Builder

    // Write the main error message if present
    if apiErr.Message != "" {
        buf.WriteString(apiErr.Message)
    }

    // Write field errors
    if len(apiErr.Errors) > 0 {
        if buf.Len() > 0 {
            buf.WriteString(":\n")
        }
        for _, fieldErr := range apiErr.Errors {
            if fieldErr.Field != "" {
                buf.WriteString(fmt.Sprintf("  - Field '%s': %s\n", fieldErr.Field, fieldErr.Message))
            } else {
                buf.WriteString(fmt.Sprintf("  - %s\n", fieldErr.Message))
            }
        }
    }

    result := buf.String()
    // Remove trailing newline
    if strings.HasSuffix(result, "\n") {
        result = result[:len(result)-1]
    }
    return result
}

// Usage in orchestrator methods:
_, err := o.apiClient.PatchLinuxInstallAppConfigValues(opts.ConfigValues)
if err != nil {
    // Check if it's an APIError with field details
    var apiErr *types.APIError
    if errors.As(err, &apiErr) && len(apiErr.Errors) > 0 {
        // Format and display the structured error
        formattedErr := formatAPIError(apiErr)
        loading.ErrorClosef("Application configuration failed")
        return fmt.Errorf("%s\n\nPlease correct the above issues and retry", formattedErr)
    }
    // Generic error handling
    loading.ErrorClosef("Application configuration failed: %v", err)
    return fmt.Errorf("patch app config values: %w", err)
}
```

**Example formatted output:**

```
Error: config validation failed:
  - Field 'database_host': required field missing
  - Field 'replica_count': value "10" exceeds maximum allowed value 5
  - Field 'enable_ssl': validation rule failed: SSL requires cert_path to be set

Please correct the above issues and retry
```

### Progress Reporter Specification

The orchestrator uses the existing `spinner.MessageWriter` from `pkg/spinner` to provide progress output:

```go
// cmd/installer/cli/headless/install/orchestrator.go

import "github.com/replicatedhq/embedded-cluster/pkg/spinner"

// orchestrator is the concrete implementation of the Orchestrator interface
type orchestrator struct {
    apiClient      client.Client
    target         apitypes.InstallTarget
    progressWriter spinner.WriteFn        // Output writer for progress messages
    logger         logrus.FieldLogger     // Logger for detailed logging
}

// Example usage in orchestrator methods:
func (o *orchestrator) runHostPreflights(ctx context.Context, ignoreFailures bool) error {
    o.logger.Debug("Starting host preflights")

    loading := spinner.Start(spinner.WithWriter(o.progressWriter))
    loading.Infof("Running host preflights...")

    // Trigger preflights
    resp, err := o.apiClient.RunLinuxInstallHostPreflights(ignoreFailures)
    if err != nil {
        loading.ErrorClosef("Host preflights failed: %v", err)
        return fmt.Errorf("run linux install host preflights: %w", err)
    }

    getStatus := func() (types.State, string, error) {
        resp, err = o.apiClient.GetLinuxInstallHostPreflightsStatus()
        if err != nil {
            return types.State{}, "", err
        }
        return resp.Status.State, resp.Status.Description, nil
    }

    // Poll for completion
    err = pollUntilComplete(ctx, getStatus)
    if err != nil {
        loading.ErrorClosef("Host preflights failed: %v", err)
        return fmt.Errorf("poll until complete: %w", err)
    }

    // Check if there are any failures in the preflight results
    hasFailures := resp.HasFailures()
    if hasFailures {
        loading.ErrorClosef("Host preflights completed with failures")

        o.logger.Warn("")
        o.logger.Warn("⚠ Warning: Host preflight checks completed with failures"))
        o.logger.Warn("")

        // Display failed checks
        for _, result := range resp.Results {
            if result.IsFail || result.IsWarn {
                level := "ERROR"
                if result.IsWarn {
                    level = "WARN"
                }
                o.logger.Warnf("  [%s] %s: %s", level, result.Title, result.Message)
            }
        }

        if ignoreFailures {
            // Display failures but continue installation
            o.logger.Warn("")
            o.logger.Warn("Installation will continue, but the system may not meet requirements (failures bypassed with flag).")
            o.logger.Warn("")
        } else {
            // Failures are not being bypassed - return error
            o.logger.Warn("")
            o.logger.Warn("Please correct the above issues and retry, or run with --ignore-host-preflights to bypass (not recommended).")
            o.logger.Warn("")

            return fmt.Errorf("host preflight checks completed with failures")
        }
    } else {
        loading.Closef("Host preflights passed")
        o.logger.Debug("Host preflights passed")
    }

    return nil
}

// Similar pattern for app preflights
func (o *orchestrator) runAppPreflights(ctx context.Context, ignoreFailures bool) error {
    o.logger.Debug("Starting app preflights")

    loading := spinner.Start(spinner.WithWriter(o.progressWriter))
    loading.Infof("Running app preflights...")

    // Trigger preflights
    resp, err := o.apiClient.RunLinuxInstallAppPreflights()
    if err != nil {
        loading.ErrorClosef("App preflights failed: %v", err)
        return fmt.Errorf("run linux install app preflights: %w", err)
    }

    getStatus := func() (types.State, string, error) {
        resp, err = o.apiClient.GetLinuxInstallAppPreflightsStatus()
        if err != nil {
            return types.State{}, "", err
        }
        return resp.Status.State, resp.Status.Description, nil
    }

    // Poll for completion
    err = pollUntilComplete(ctx, getStatus)
    if err != nil {
        loading.ErrorClosef("App preflights failed: %v", err)
        return fmt.Errorf("poll until complete: %w", err)
    }

    // Check if there are any failures in the preflight results
    hasFailures := resp.HasFailures()
    if hasFailures {
        loading.ErrorClosef("App preflights completed with failures")

        o.logger.Warn("")
        o.logger.Warn("⚠ Warning: Application preflight checks completed with failures"))
        o.logger.Warn("")

        // Display failed checks
        for _, result := range resp.Results {
            if result.IsFail || result.IsWarn {
                level := "ERROR"
                if result.IsWarn {
                    level = "WARN"
                }
                o.logger.Warnf("  [%s] %s: %s", level, result.Title, result.Message)
            }
        }

        if ignoreFailures {
            // Display failures but continue installation
            o.logger.Warn("")
            o.logger.Warn("Installation will continue, but the application may not function correctly (failures bypassed with flag).")
            o.logger.Warn("")
        } else {
            // Failures are not being bypassed - return error
            o.logger.Warn("")
            o.logger.Warn("Please correct the above issues and retry, or run with --ignore-app-preflights to bypass (not recommended).")
            o.logger.Warn("")
            return fmt.Errorf("app preflight checks completed with failures")
        }
    } else {
        loading.Closef("App preflights passed")
        o.logger.Debug("App preflights passed")
    }

    return nil
}
```

**spinner.MessageWriter API:**

- `spinner.Start(spinner.WithWriter(o.progressWriter))` - Creates new spinner with orchestrator's progress writer
- `Infof(msg string, args...)` - Updates spinner message (shows rotating spinner symbol)
- `Closef(msg string, args...)` - Shows success with checkmark `✔` and stops spinner
- `ErrorClosef(msg string, args...)` - Shows error with X `✗` and stops spinner

**Expected output format:**

```
$ embedded-cluster install --headless --target linux --license license.yaml --config-values config.yaml --admin-console-password password --yes

◓  Initializing
✔  Initialization complete
◓  Running host preflights...
✔  Host preflights passed
◓  Setting up infrastructure...
◓  Infrastructure: Installing K0s
◓  Infrastructure: Waiting for control plane
✔  Node is ready
✔  Storage is ready
✔  Runtime Operator is ready
✔  Disaster Recovery is ready
✔  Installing additional components (2/2)
✔  Installation complete

Installation completed successfully
```

### HTTP Client Configuration

The in-process API server uses TLS certificates (either auto-generated or provided via `--tls-cert`/`--tls-key` flags). For localhost connections, the HTTP client is configured in `buildOrchestrator()` with `InsecureSkipVerify` to simplify the implementation:

```go
// cmd/installer/cli/install_v3.go

func buildOrchestrator(
    installCfg *installConfig,
    apiOpts apiOptions,
) (install.Orchestrator, error) {
    // Construct API URL from manager port
    apiURL := fmt.Sprintf("https://localhost:%d", installCfg.managerPort)

    // Validate target
    if apiOpts.InstallTarget != apitypes.InstallTargetLinux {
        return nil, fmt.Errorf("%s target not supported", apiOpts.InstallTarget)
    }

    // Create HTTP client with InsecureSkipVerify for localhost
    // Since the API server is in-process and on localhost only, certificate
    // validation is not critical for this use case
    httpClient := &http.Client{
        Timeout: 30 * time.Second,
        Transport: &http.Transport{
            Proxy: nil,  // No proxy for localhost
            TLSClientConfig: &tls.Config{
                InsecureSkipVerify: true,  // Acceptable for localhost in-process API
            },
        },
    }

    // Create API client
    apiClient := client.New(
        apiURL,  // e.g., "https://localhost:30000"
        client.WithHTTPClient(httpClient),
    )

    // Create orchestrator with the configured API client
    orchestrator, err := install.NewOrchestrator(
        apiClient,
        apiOpts.Password,
        apiOpts.InstallTarget,
    )
    if err != nil {
        return nil, fmt.Errorf("failed to create orchestrator: %w", err)
    }

    return orchestrator, nil
}
```

**Rationale**: We use `InsecureSkipVerify: true` for the localhost-only in-process API server. Since the API server runs in the same process and only listens on localhost, certificate validation is not necessary. This approach simplifies the implementation by avoiding certificate parsing and management, and makes testing easier by allowing the API client to be injected into `NewOrchestrator`.

**Important**: The API URL should be constructed as `https://localhost:<manager-port>` where `manager-port` defaults to 30000 but can be overridden with `--manager-port` flag.

### Toggle Strategy

Feature flag: **Not needed** - Controlled by CLI flag

- `--headless` flag explicitly enables headless mode
- Absence of flag uses existing UI flow
- No runtime feature flag or entitlement required

### External Contracts

No changes to external APIs or events. The headless orchestrator uses the same internal API that the UI uses.

## Testing

### "Dry-Run" Integration Tests

A dry-run test suite exists in the current project today. It allows for testing the installer's behavior without actually executing any system-modifying commands or making real API calls. The tests are portable (go tests that rely only on the docker runtime), reliable (mock all external APIs and services), and quick (the entire suite runs in under 1 minute).

These tests validate the installer's logic by capturing and inspecting what would be executed during real operations, including:

- Commands that would run
- Helm charts that would be installed/upgraded
- Kubernetes resources that would be created
- Environment variables that would be set
- Host preflight checks that would be performed
- Metrics that would be sent

We will rely exclusively on dry-run tests for input and validation logic.

### Concrete Test Scenarios

All tests follow the **happy path** pattern - verifying that flag configurations propagate correctly through the entire installation. Tests will be added to `tests/dryrun/v3_install_headless_test.go`.

Each test validates that the flag value flows through to all relevant components: K0s config, Helm addon values, host preflights, environment variables, and commands.

**Test Pattern:**

```go
func TestV3HeadlessInstall_<FlagName>(t *testing.T) {
    // GIVEN: Headless install with specific flag configuration
    dr := dryrunInstall(t, client,
        "--headless",
        "--target", "linux",
        "--config-values", configValuesFile,
        "--admin-console-password", "password",
        "--<flag>", "<value>",
        "--yes",
    )

    // THEN: Verify flag propagates to all relevant components
    // - K0s configuration
    // - Addon Helm values
    // - Host preflights
    // - Environment variables
    // - Commands
}
```

---

**1. HTTP Proxy Configuration** (`--http-proxy`, `--https-proxy`, `--no-proxy`)

```go
func TestV3HeadlessInstall_HTTPProxyPropagation(t *testing.T) {
    // GIVEN: Headless install with proxy configuration
    //        --http-proxy http://proxy.example.com:3128
    //        --https-proxy https://proxy.example.com:3128
    //        --no-proxy localhost,127.0.0.1,10.0.0.0/8
    // WHEN: Installation is run via dryrunInstall
    // THEN: Proxy settings propagate to:
    //   - K0s config (spec.extensions.helm.repositories[].proxySpec)
    //   - Embedded Cluster Operator addon (extraEnv: HTTP_PROXY, HTTPS_PROXY, NO_PROXY)
    //   - Velero addon (configuration.extraEnvVars)
    //   - Registry addon (extraEnvVars)
    //   - Admin Console addon (extraEnv: HTTP_PROXY, HTTPS_PROXY, NO_PROXY)
    //   - Host preflights (environment variables set for collectors)
    //   - App preflights (environment variables)
    //   - KOTS CLI receives proxy environment variables (HTTP_PROXY, HTTPS_PROXY, NO_PROXY)
    //   - NO_PROXY is calculated to include cluster CIDRs and service CIDRs
}
```

**2. Network Configuration** (`--network-interface`, `--pod-cidr`, `--service-cidr`)

```go
func TestV3HeadlessInstall_NetworkConfiguration(t *testing.T) {
    // GIVEN: Headless install with custom network settings
    //        --network-interface eth1
    //        --pod-cidr 10.244.0.0/16
    //        --service-cidr 10.245.0.0/16
    // WHEN: Installation is run via dryrunInstall
    // THEN: Network settings propagate to:
    //   - K0s config (spec.network.podCIDR, spec.network.serviceCIDR)
    //   - RuntimeConfig uses specified network interface
    //   - Host preflights check the correct network interface
    //   - NO_PROXY includes the specified CIDRs
    //   - Embedded Cluster Operator has correct CIDR env vars
}
```

**3. Data Directory Configuration** (`--data-dir`)

```go
func TestV3HeadlessInstall_DataDirectory(t *testing.T) {
    // GIVEN: Headless install with custom data directory
    //        --data-dir /custom/data/path
    // WHEN: Installation is run via dryrunInstall
    // THEN: Data directory propagates to:
    //   - K0s config (spec.storage.etcd.dataDirs)
    //   - RuntimeConfig.EmbeddedClusterHomeDirectory
    //   - Admin Console addon (embeddedClusterDataDir, embeddedClusterK0sDir)
    //   - Host preflights check disk space at custom path
    //   - All addon persistent volumes use custom path as base
    //   - Backup/restore configurations reference custom path
}
```

**4. Local Artifact Mirror Port** (`--local-artifact-mirror-port`)

```go
func TestV3HeadlessInstall_LocalArtifactMirrorPort(t *testing.T) {
    // GIVEN: Headless install with custom artifact mirror port
    //        --local-artifact-mirror-port 8082
    // WHEN: Installation is run via dryrunInstall
    // THEN: Port configuration propagates to:
    //   - Registry addon Helm values (service.port)
    //   - K0s config references correct port for image pulls
    //   - Embedded Cluster Operator env vars (LOCAL_ARTIFACT_MIRROR_PORT)
    //   - RuntimeConfig.LocalArtifactMirrorPort
}
```

**5. Admin Console Port** (`--admin-console-port`)

```go
func TestV3HeadlessInstall_AdminConsolePort(t *testing.T) {
    // GIVEN: Headless install with custom admin console port
    //        --admin-console-port 8081
    // WHEN: Installation is run via dryrunInstall
    // THEN: Port configuration propagates to:
    //   - Admin Console addon Helm values (kurlProxy.nodePort)
    //   - KOTS admin console service (targetPort)
    //   - Ingress/NodePort configuration
    //   - Host preflights check port availability
    //   - Installation success message shows correct port in URL
}
```

**6. Airgap Installation** (`--airgap-bundle`)

```go
func TestV3HeadlessInstall_AirgapBundle(t *testing.T) {
    // GIVEN: Headless install with airgap bundle
    //        --airgap-bundle /path/to/bundle.airgap
    // WHEN: Installation is run via dryrunInstall
    // THEN: Airgap configuration propagates to:
    //   - K0s config uses embedded images (no external registries)
    //   - Local artifact mirror is configured and populated
    //   - All addon Helm values reference local registry
    //   - Admin Console addon (isAirgap: true)
    //   - Host preflights skip internet connectivity checks
    //   - App preflights use airgap mode
    //   - KOTS CLI is called with --airgap-bundle flag
    //   - NO_PROXY is not auto-expanded with external endpoints
}
```

**7. Config Values Propagation** (`--config-values`)

```go
func TestV3HeadlessInstall_ConfigValuesPropagation(t *testing.T) {
    // GIVEN: Headless install with config values file containing:
    //        - hostname: myapp.example.com
    //        - replica_count: 3
    //        - database_host: postgres.example.com
    // WHEN: Installation is run via dryrunInstall
    // THEN: Config values propagate to:
    //   - KOTS CLI is called with --config-values flag
    //   - Application Helm chart values
    //   - App preflights use config values for connectivity checks
    //   - ConfigMap created in kotsadm namespace with values
    //   - Application deployments reflect configured values
}
```

**8. TLS Certificate Configuration** (`--tls-cert`, `--tls-key`, `--hostname`)

```go
func TestV3HeadlessInstall_CustomTLSCertificate(t *testing.T) {
    // GIVEN: Headless install with custom TLS configuration
    //        --tls-cert /path/to/cert.pem
    //        --tls-key /path/to/key.pem
    //        --hostname myapp.example.com
    // WHEN: Installation is run via dryrunInstall
    // THEN: TLS configuration propagates to:
    //   - Admin Console addon creates TLS secret with provided cert/key
    //   - KOTS admin console uses provided certificate
    //   - Ingress resources reference correct secret
    //   - Certificate is stored in appropriate namespace
    //   - Installation URL uses specified hostname
}
```

**9. Preflight Bypass** (`--ignore-host-preflights`, `--ignore-app-preflights`)

```go
func TestV3HeadlessInstall_PreflightBypass(t *testing.T) {
    // GIVEN: Headless install with preflight bypass flags
    //        --ignore-host-preflights
    //        --ignore-app-preflights
    // WHEN: Installation is run via dryrunInstall
    // THEN: Bypass flags are respected:
    //   - Host preflights run but failures don't block installation
    //   - App preflights run but failures don't block deployment
    //   - Warnings are logged about bypassed checks
    //   - Infrastructure setup proceeds despite preflight warnings
    //   - Metrics track that preflights were bypassed
}
```

### E2E Tests

We do not yet have end-to-end tests for the V3 installer. These are out of the scope of this proposal and we will revisit tests in a followup.

### Test Data/Fixtures

Example test assets created in `tests/dryrun/assets/`:

```yaml
# tests/dryrun/assets/kotskinds-config.yaml - Config schema definition
apiVersion: kots.io/v1beta1
kind: Config
spec:
  groups:
    - name: config_group
      title: The First Config Group
      items:
        - name: hostname
          title: Hostname
          type: text
        - name: pw
          title: Password
          type: password
```

```yaml
# Example ConfigValues file for headless install (provided by user)
apiVersion: kots.io/v1beta1
kind: ConfigValues
metadata:
  name: my-app-config
spec:
  values:
    hostname:
      value: "postgres.example.com"
    pw:
      value: "secretpassword"
```

## Backward compatibility

### API Versioning

- No API version changes required
- Uses existing v3 API endpoints
- No breaking changes to API contracts

### Data Format Compatibility

- Supports existing `kots.io/v1beta1 ConfigValues` format
- No changes to config value structure
- Existing config files work unchanged

### Migration Windows

- V2 headless installs continue to work
- No migration required for existing installations
- New headless mode is opt-in via `--headless` flag

## Migrations

**No special deployment handling required.**

This feature is additive and doesn't require any migrations. The deployment process is standard:

1. Deploy new embedded-cluster binary with headless support
2. Users can immediately use `--headless` flag
3. Existing installations are unaffected

## Trade-offs

**Optimizing for**: Consistency, maintainability, and user experience

1. **In-Process API vs Separate API Server**

   - Chosen: Run API server in-process within the CLI
   - Trade-off: Cannot support remote installations where CLI runs on a different machine from the installation target
   - Benefit: Simpler architecture with no daemon management, no network configuration complexity, no remote authentication concerns. API server lifecycle is tied to CLI process. Eliminates entire class of issues around server startup, port conflicts, and process management.

2. **API-based vs Direct Controller Calls**

   - Chosen: API-based approach using HTTP client to call v3 API endpoints (in-process)
   - Trade-off: Requires authentication and HTTP overhead (albeit in-process/localhost)
   - Benefit: Maintains single code path between UI and headless flows. Controllers are designed for HTTP invocation with request/response objects. Direct controller calls would require duplicating validation, error handling, and response formatting logic, creating divergent code paths. The API layer exists purely as an abstraction for consistency, not as a remote service.

3. **Explicit --headless flag vs Inference**

   - Chosen: Explicit `--headless` flag
   - Trade-off: Extra flag required
   - Benefit: Clear user intent, no ambiguity

4. **Synchronous vs Async Operations**

   - Chosen: Synchronous with progress reporting
   - Trade-off: CLI blocks until complete
   - Benefit: Simple scripting, clear success/failure

## Validation Strategy

Validation happens at multiple stages to fail fast when possible:

1. **CLI Flag Validation** (immediate, before API starts):

   - Required flags present: `--config-values`, `--admin-console-password`
   - Config values file exists and is readable
   - YAML syntax is valid

2. **Config Values Schema Validation** (early, via API):

   - Validate against vendor's `kots.io/v1beta1.Config` schema
   - Check required fields are present
   - Validate field types and constraints
   - Run validation rules defined in Config spec

3. **Installation Settings Validation** (early, via API):

   - Network CIDR ranges don't conflict
   - Ports do not conflict
   - Network interface is valid

4. **Host Preflight Validation** (runtime, can be bypassed):

   - System requirements checks
   - Can be skipped with `--ignore-host-preflights`

5. **App Preflight Validation** (runtime, after infra setup, can be bypassed):

   - Application-specific checks
   - Can be skipped with `--ignore-app-preflights`

**Point of No Return**: Once infrastructure installation begins (K0s setup), the system cannot be resumed. Any failure requires `embedded-cluster reset` and full reinstall.

## Failure Modes and Recovery

### Config Value Parse Failures

**When**: During CLI startup, before API starts

**Example**:

```
Error: failed to load config values from ./config.yaml: yaml: line 5: mapping values are not allowed in this context
```

**Recovery**: Fix YAML syntax and re-run install command. No reset needed.

### Config Value Validation Failures

**When**: After submitting to API, before any installation steps

**Example**:

```
◓  Initializing

Error: config values validation failed:
  - Field 'database_host': required field missing
  - Field 'replica_count': value "10" exceeds maximum allowed value 5
  - Field 'enable_ssl': validation rule failed: SSL requires cert_path to be set

Please correct the above issues and retry
```

**Note**: This error is formatted by the `formatAPIError()` helper function, which extracts field-level errors from the APIError structure returned by `PatchLinuxInstallAppConfigValues()`.

**Recovery**: Fix config values and re-run install command. No reset needed.

### Installation Settings Validation Failures

**When**: After config values validation, before any installation steps

**Example**:

```
◓  Initializing

Error: installation settings validation failed:
  - Pod CIDR 10.96.0.0/12 overlaps with service CIDR 10.96.0.0/16
  - Network interface 'eth1' not found on host

For configuration options, run: embedded-cluster install --help
Please correct the above issues and retry
```

**Note**: This error is formatted by the `formatAPIError()` helper function when the API returns validation errors from `ConfigureLinuxInstallation()`.

**Recovery**: Fix network settings via flags and re-run install command. No reset needed.

### Host Preflight Failures

**When**: Before infrastructure setup (can be bypassed)

**Example (not bypassed)**:

```
✔  Initialization complete
✗  Host preflights completed with failures

⚠ Warning: Host preflight checks completed with failures

  - [ERROR] Insufficient disk space: 10GB available, 50GB required
  - [WARN] CPU count below recommended: 2 cores, 4 recommended

Please correct the above issues and retry, or run with --ignore-host-preflights to bypass (not recommended)
```

**Recovery**: Fix host issues or re-run with `--ignore-host-preflights`. No reset needed.

**Example (bypassed with --ignore-host-preflights)**:

```
✔  Initialization complete
✗  Host preflights completed with failures

⚠ Warning: Host preflight checks completed with failures

  [ERROR] Insufficient disk space: 10GB available, 50GB required
  [WARN] CPU count below recommended: 2 cores, 4 recommended

Installation will continue, but the system may not meet requirements.

✔  Node is ready
...
```

**Behavior**: When `--ignore-host-preflights` is specified:

- Preflight checks run to completion
- Spinner closes with error status and "completed with failures" message
- Failures are displayed with warning message via logger
- Installation continues despite failures
- User is warned that system may not meet requirements
- Failures are logged to CLI and API logs

### Infrastructure Setup Failures (Point of No Return)

**When**: During K0s/infrastructure installation

**Example**:

```
✔  Initialization complete
✔  Host preflights passed
◓  Installing node

Error: Node installation failed: K0s failed to start: context deadline exceeded

To collect diagnostic information, run: embedded-cluster support-bundle
To retry installation, run: embedded-cluster reset and wait for server reboot
```

**Recovery**: Must run `embedded-cluster reset` and re-run full install.

### App Preflight Failures

**When**: After infrastructure is running, before app install (can be bypassed)

**Example (not bypassed)**:

```
✔  Initialization complete
✔  Host preflights passed
✔  Node is ready
✔  Storage is ready
✔  Runtime Operator is ready
✔  Disaster Recovery is ready
✔  Admin Console is ready
✔  Additional components are ready
◓  Running application preflights

Error: Application preflight checks completed with failures:
  - [ERROR] Cannot connect to required database host postgres.example.com:5432
  - [WARN] PVC storage class 'fast' not available

To collect diagnostic information, run: embedded-cluster support-bundle
To retry installation, run: embedded-cluster reset and wait for server reboot
```

**Recovery**: Must run `embedded-cluster reset` and re-run full install.

**Example (bypassed with --ignore-app-preflights)**:

```
✔  Initialization complete
✔  Host preflights passed
✔  Node is ready
✔  Storage is ready
✔  Runtime Operator is ready
✔  Disaster Recovery is ready
✔  Admin Console is ready
✔  Additional components are ready
✗  App preflights completed with failures

⚠ Warning: Application preflight checks completed with failures

  [ERROR] Cannot connect to required database host postgres.example.com:5432
  [WARN] PVC storage class 'fast' not available

Installation will continue, but the application may not function correctly (failures bypassed with flag).

✔  Application is ready
✔  Installation complete
```

**Behavior**: When `--ignore-app-preflights` is specified:

- Preflight checks run to completion
- Spinner closes with error status and "completed with failures" message
- Failures are displayed with warning message via logger
- Installation continues despite failures
- User is warned that application may not function correctly
- Failures are logged to CLI and API logs

### Application Installation Failures

**When**: Final installation phase

**Example**:

```
✔  Initialization complete
✔  Host preflights passed
✔  Node is ready
✔  Storage is ready
✔  Runtime Operator is ready
✔  Disaster Recovery is ready
✔  Admin Console is ready
✔  Additional components are ready
◓  Installing application

Error: Application installation failed: timeout waiting for pods to become ready

To collect diagnostic information, run: embedded-cluster support-bundle
To retry installation, run: embedded-cluster reset and wait for server reboot
```

**Recovery**: Must run `embedded-cluster reset` and re-run full install.

## Reset and Recovery Experience

### When Reset is Required

Reset is required when installation fails **after infrastructure setup begins**. This is the point of no return because K0s is running and has created cluster state.

**What's Removed**:

- All K0s cluster state
- Container runtime data
- Network configuration (CNI, iptables)
- Installation state in Manager DB

**What Persists**:

- Downloaded embedded-cluster binary
- License file (if saved locally)
- Config values file (if saved locally)
- System logs in `/var/log/embedded-cluster/`

### Recovery Workflow

1. **Identify the failure** from error output and support bundle
2. **Fix the root cause** (update config values, fix host issues, correct network settings)
3. **Run reset**: `embedded-cluster reset`
4. **Re-run install**: `embedded-cluster install --headless [flags]`

**Example full recovery**:

```bash
# Initial install fails due to bad config
$ embedded-cluster install --headless --target linux --config-values config.yaml --license license.yaml --admin-console-password pass123
✔  Initialization complete
✔  Host preflights passed
◓  Installing node

Error: Node installation failed: K0s failed to start: context deadline exceeded

# Reset the system
$ embedded-cluster reset

✔  Reset complete

# Fix config.yaml with correct values
$ vim config.yaml

# Retry installation
$ embedded-cluster install --headless --target linux --config-values config.yaml --license license.yaml --admin-console-password pass123

✔  Initialization complete
✔  Host preflights passed
✔  Node is ready
✔  Storage is ready
✔  Runtime Operator is ready
✔  Disaster Recovery is ready
✔  Admin Console is ready
✔  Additional components are ready
✔  Application preflights passed
✔  Application is ready
✔  Installation complete
```

### Why Resumption Isn't Supported (v1)

Resumption adds significant complexity:

- State recovery logic for each installation phase
- Handling partial K0s installations
- Managing interrupted airgap uploads
- Determining safe resume points vs. requiring reset

Initial implementation prioritizes reliability and debuggability. Future iterations may add resumption for specific scenarios.

## Debuggability and Observability

### Logging

All headless install operations log to:

- **Console**: Progress and errors (user-facing)
- **File**: `/var/log/embedded-cluster/embedded-cluster-TIMESTAMP.api.log` (detailed debug logs)
- **Journal**: `journalctl -u k0scontroller.service` (K0s logs)

**Example log output**:

```
2025-10-21T10:30:15Z INFO  Headless install started target=linux
2025-10-21T10:30:15Z DEBUG Config values loaded from ./config.yaml
2025-10-21T10:30:16Z INFO  Authentication successful
2025-10-21T10:30:17Z DEBUG Patching config values via API endpoint=/api/v3/linux/install/app/config-values
2025-10-21T10:30:18Z INFO  Config validation passed
2025-10-21T10:30:20Z ERROR Infrastructure setup failed error="K0s failed to start" exit_code=3
```

### Error Message Format

All errors include:

- **What failed**: Clear description of the failure
- **Why it failed**: Root cause when available
- **How to fix**: Specific recovery steps

Errors follow the format established by the existing installation output, using clear error messages with actionable guidance.

## Alternative solutions considered

### 1. Direct Controller Invocation

Skip the API layer and directly call controller functions from the CLI, bypassing HTTP entirely.

- **Rejected**: Creates divergent code paths between UI and headless flows. Controllers are designed to be invoked via HTTP handlers with request/response objects. Calling them directly would require duplicating request validation, error handling, and response formatting logic. Additionally, the state machine and managers expect to be accessed through the API layer, so bypassing it would require significant refactoring and lose the benefits of a single, consistent code path.

  The primary use case for headless installs is automation on the target host itself (CI/CD, testing, scripting). Remote installation scenarios can be addressed by running the CLI on the target host (via SSH or similar). The in-process approach eliminates an entire class of operational complexity while serving the core use case effectively.

## Research

### Prior Art in Codebase

1. **V2 Headless Implementation** (`cmd/installer/cli/install.go`):

   - Uses `isHeadlessInstall := flags.configValues != "" && flags.adminConsolePassword != ""`
   - Directly calls KOTS CLI
   - [Link to code](file://cmd/installer/cli/install.go#L840)

2. **V3 State Machine** (`api/internal/statemachine/`):

   - Manages installation state transitions
   - [Research document](file://proposals/headless_install_research.md)

3. **API Authentication** (`api/controllers/auth/`):

   - JWT-based authentication system
   - [Link to code](file://api/controllers/auth/controller.go)

### External References

1. **KOTS Headless Installs**:

   - Similar pattern of config values + password
   - [KOTS Documentation](https://docs.replicated.com/enterprise/installing-embedded-cluster)

2. **Industry Standards**:

   - Kubernetes operators often use similar patterns
   - Helm uses values.yaml for configuration
   - Ansible uses inventory files

## Checkpoints (PR plan)

This will be implemented in **multiple PRs** for easier review:

### PR 1: CLI Flags and Validation

- Add `--headless` flag
- Add validation for headless requirements (flags, YAML parsing, file existence)
- Add config value loading
- Unit tests for flag validation and error cases

### PR 2: Headless Orchestrator Implementation

- Implement `headless/install.Orchestrator` struct
- Add authentication logic
- Add API interaction methods
- Implement error handling and recovery messaging
- Add progress reporting with error formatting
- Unit tests for client and error scenarios
- Add config values schema validation via API
- Add installation settings validation
- Add failure detection and structured error responses
- Integration tests for happy and failure paths
