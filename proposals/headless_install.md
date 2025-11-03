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
Report success/failure
    ↓
API server shuts down when CLI exits
```

### Key Components

1. **In-Process API Server**: The v3 API server runs within the CLI process itself, not as a separate daemon. This provides an abstraction layer that maintains consistency with browser-based installations while keeping everything in a single process.
2. **headless/install.Orchestrator**: High-level orchestration wrapper around the existing `api/client.Client` that manages automatic state progression and progress reporting. Makes in-process HTTP calls to the API server.
3. **api/client.Client**: Existing low-level HTTP client (already implemented at `api/client/client.go`) that provides methods to interact with v3 API endpoints via standard HTTP calls (localhost).
4. **ProgressReporter**: Reports progress to CLI output, matching the existing v2 output format with checkmarks and spinners.

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
- Backwards compatibility with v2 headless install semantics (v2 continues to work separately)

## Database

**No database changes required.**

The headless installation uses the same underlying storage mechanisms as the UI-based installation through the existing API and state management infrastructure.

## Implementation plan

**Note on Architecture**: The API server will be started in-process within the CLI when `--headless` flag is used. The server runs on localhost only and shuts down when the CLI exits. All API interactions happen via HTTP calls to localhost within the same process.

### Files/Services to Touch

1. **CLI Layer**:
   - `cmd/installer/cli/install.go`: Add headless mode logic, start in-process API server, front-loaded validation (flags, YAML parsing)
   - `cmd/installer/cli/flags.go`: Add `--headless` flag
   - `cmd/installer/cli/headless/install/install.go`: NEW - Orchestration of API interaction to mimic browser based install (makes localhost HTTP calls to in-process API)
   - `cmd/installer/cli/headless/install/progress.go`: NEW - Progress reporting and error formatting
   - `cmd/installer/cli/headless/install/validation.go`: NEW - CLI-level validation (flags, YAML syntax)

2. **API Layer** (runs in-process):
   - `api/controllers/auth/controller.go`: Add programmatic token generation for in-process authentication
   - `api/controllers/app/controller.go`: Remove early config value validation from `NewAppController` ([lines 179-188](https://github.com/replicatedhq/embedded-cluster/blob/788c252330064ee592c401dcc512a486ac93d303/api/controllers/app/controller.go#L188-L191)). Defer validation for browser based and headless installs until the API calls are made in the Browser and corresponding `RunHeadlessInstall` function to make install paths consistent.
   - `api/handlers.go`: Ensure handlers work with headless orchestrator, return structured errors
   - `api/server.go`: Ensure server can be started/stopped cleanly within CLI process lifecycle

3. **State Management**:
   - `api/internal/statemachine/auto_progress.go`: NEW - Auto-progression logic with failure detection

### Pseudo Code

```go
// cmd/installer/cli/headless/install/install.go
type Orchestrator struct {
    apiClient client.Client
    target    apitypes.InstallTarget // "linux" or "kubernetes"
    reporter  *ProgressReporter
}

type HeadlessInstallOptions{
    ConfigValues            apitypes.ConfigValues
    LinuxInstallationConfig apitypes.LinuxInstallationConfig
    IgnoreHostPreflights    bool
    IgnoreAppPreflights     bool
    AirgapBundle            string
}

func NewOrchestrator(baseURL string, password string, target string) (*Orchestrator, error) {
    // We do not yet support the "kubernetes" target
    if target != apitypes.InstallTargetLinux {
        return nil, fmt.Errorf("%s target not supported", target)
    }

    // Create api/client.Client instance
    apiClient := client.New(baseURL)

    // Authenticate and set token
    if err := apiClient.Authenticate(password); err != nil {
        return nil, fmt.Errorf("authentication failed: %w", err)
    }

    return &Orchestrator{
        apiClient: apiClient,
        target:    target,
        reporter:  NewProgressReporter(),
    }, nil
}

func (o *Orchestrator) RunHeadlessInstall(ctx context.Context, opts HeadlessInstallOptions) error {
    // Configure application with config values
    if err := o.configureApplication(ctx, opts); err != nil {
        return err
    }

    // Configure installation
    if err := o.configureInstallation(ctx, opts); err != nil {
        return err
    }

    // Run host preflights (allow bypass when --ignore-host-preflights flag is set)
    if err := o.runHostPreflights(ctx, opts.IgnoreHostPreflights); err != nil {
        return err
    }

    // Setup infrastructure
    if err := o.setupInfrastructure(ctx, opts.IgnoreHostPreflights); err != nil {
        return err
    }

    // Process airgap if needed
    if opts.AirgapBundle != "" {
        if err := o.processAirgap(ctx); err != nil {
            return err
        }
    }

    // Run app preflights (allow bypass when --ignore-app-preflights flag is set)
    if err := o.runAppPreflights(ctx, opts.IgnoreAppPreflights); err != nil {
        return err
    }

    // Install application
    return o.installApp(ctx, opts.IgnoreAppPreflights)
}

// Example method showing how Client wraps api/client.Client
func (o *Orchestrator) configureApplication(ctx context.Context, opts HeadlessOptions) error {
    o.reporter.Progress("Loading config values...")

    // Use the wrapped api/client.Client to patch config values
    o.reporter.Progress("Submitting config values to API...")
    _, err = o.apiClient.PatchLinuxInstallAppConfigValues(opts.ConfigValues)
    if err != nil {
        return fmt.Errorf("failed to submit config values: %w", err)
    }

    o.reporter.Success("Config values configured successfully")
    return nil
}

// Another example showing state monitoring
func (o *Orchestrator) setupInfrastructure(ctx context.Context, ignoreHostPreflights bool) error {
    o.reporter.Progress("Setting up infrastructure...")

    // Initiate infra setup using api/client.Client
    infra, err := o.apiClient.SetupLinuxInfra(ignoreHostPreflights)
    if err != nil {
        return fmt.Errorf("failed to setup infrastructure: %w", err)
    }

    // Poll for completion using api/client.Client
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-time.After(5 * time.Second):
            infra, err = o.apiClient.GetLinuxInfraStatus()
            if err != nil {
                return fmt.Errorf("failed to get infra status: %w", err)
            }

            if infra.Status == "complete" {
                o.reporter.Success("Infrastructure setup complete")
                return nil
            } else if infra.Status == "error" {
                return fmt.Errorf("infrastructure setup failed: %s", infra.Error)
            }

            o.reporter.Progress(fmt.Sprintf("Infrastructure setup in progress: %s", infra.Status))
        }
    }
}
```

### New Handlers/Controllers

No new API handlers or controllers needed. Existing endpoints will be used:
- `/api/auth/login` - Authentication
- `/api/linux/install/*` - Linux installation endpoints

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

Following is a list of tests that will be added as dry-run tests:

- Test headless install with various config values
- Test headless install with various apitypes.LinuxInstallationConfig set via flags
- Test preflight bypass scenarios
- Test config value parsing errors (invalid YAML)
- Test config value validation errors (schema violations, missing required fields)
- Test installation settings validation errors (network conflicts)
- Test host preflight failures (with and without bypass)
- Test infrastructure setup failures
- Test app preflight failures (with and without bypass)
- Test app installation failures
- Test previous installation detection

### E2E Tests

We do not yet have end-to-end tests for the V3 installer. These are out of the scope of this proposal and we will revisit tests in a followup.

### Test Data/Fixtures

```yaml
# test/fixtures/headless-config.yaml
apiVersion: kots.io/v1beta1
kind: ConfigValues
metadata:
  name: test-config
spec:
  values:
    database_host:
      value: "postgres.example.com"
    database_password:
      value: "secretpassword"
    enable_feature_x:
      value: "true"
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

**Recovery**: Fix network settings via flags and re-run install command. No reset needed.

### Host Preflight Failures

**When**: Before infrastructure setup (can be bypassed)

**Example**:
```
✔  Initialization complete
◓  Running host preflights

Error: Host preflight checks failed:
  - [ERROR] Insufficient disk space: 10GB available, 50GB required
  - [WARN] CPU count below recommended: 2 cores, 4 recommended

Please correct the above issues and retry, or run with --ignore-host-preflights to bypass (not recommended)
```

**Recovery**: Fix host issues or re-run with `--ignore-host-preflights`. No reset needed.

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
◓  Running application preflights

Error: Application preflight checks failed:
  - [ERROR] Cannot connect to required database host postgres.example.com:5432
  - [WARN] PVC storage class 'fast' not available

Please correct the above issues and retry, or run with --ignore-app-preflights to bypass (not recommended)
To retry installation, run: embedded-cluster reset and wait for server reboot
```

**Recovery**: Must run `embedded-cluster reset` and re-run full install.

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
