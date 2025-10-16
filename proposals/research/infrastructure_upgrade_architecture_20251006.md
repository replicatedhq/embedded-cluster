---
date: 2025-10-06T16:03:06Z
researcher: salah
git_commit: 556849ee21102d6de359235cbd054d0db86c1e57
branch: salah/sc-129708/template-execution-fails-when-empty-user
repository: replicatedhq/embedded-cluster
topic: "Infrastructure Upgrade Architecture in Embedded Cluster"
tags: [research, codebase, upgrade, operator, v3, autopilot, helm, addons]
status: complete
last_updated: 2025-10-06
last_updated_by: salah
---

# Research: Infrastructure Upgrade Architecture in Embedded Cluster

**Date**: 2025-10-06T16:03:06Z
**Researcher**: salah
**Git Commit**: 556849ee21102d6de359235cbd054d0db86c1e57
**Branch**: salah/sc-129708/template-execution-fails-when-empty-user
**Repository**: replicatedhq/embedded-cluster

## Research Question

Comprehensive analysis of the current infrastructure upgrade architecture in embedded-cluster, focusing on:
1. Operator-based upgrade flow and job/pod implementation
2. Artifact distribution and local artifact mirror usage
3. K0s autopilot integration for k8s upgrades
4. Addon upgrades via Helm
5. V3 architecture patterns and upgrade CLI command
6. Integration points for infrastructure upgrade
7. Code organization and key packages

Focus: Single-node online installations (note multi-node/airgap components without deep dive).

## Summary

Embedded Cluster uses a **two-tiered upgrade system**:

1. **Operator-Based Infrastructure Upgrade (Current/Legacy)**: Upgrades k0s, addons, and extensions via an in-cluster operator that creates upgrade jobs
2. **V3 App Upgrade (New)**: Out-of-cluster wizard-based upgrade flow for KOTS applications via manager UI

The operator-based flow orchestrates infrastructure changes through:
- **Artifact distribution**: Jobs on each node pull binaries/images from local artifact mirror
- **K0s autopilot**: Automated k8s version upgrades via autopilot plans
- **Helm-based addon upgrades**: Sequential upgrade of system addons (OpenEBS, Registry, Velero, Admin Console, EC Operator)
- **Upgrade job**: Coordinates the full upgrade sequence

The V3 architecture provides a foundation for **future infrastructure upgrades** through API/UI, reusing install patterns.

## Detailed Findings

### 1. Current Operator-Based Upgrade Flow

#### Operator Update Job/Pod Implementation

**File**: `operator/pkg/upgrade/job.go`

The upgrade flow is triggered by creating an **Upgrade Job** in the cluster:

```go
// CreateUpgradeJob creates a job that upgrades the embedded cluster
func CreateUpgradeJob(ctx context.Context, cli client.Client, rc runtimeconfig.RuntimeConfig,
    in *ecv1beta1.Installation, localArtifactMirrorImage, licenseID, appSlug, channelID,
    appVersion string, previousInstallVersion string) error
```

**Key steps**:
1. Copy version metadata to cluster
2. **Distribute artifacts** to all nodes (binaries, images, Helm charts)
3. Create ConfigMap with target Installation spec
4. Create upgrade Job that runs operator image with `upgrade-job` command

**Job definition** (`operator/pkg/upgrade/job.go:142-241`):
- Runs as ServiceAccount: `kotsadm-kotsadm`
- Mounts host data directory and charts directory
- Command: `/manager upgrade-job --installation /config/installation.yaml --previous-version <version>`
- Prefers scheduling on control plane nodes (affinity weight: 100)
- Pull policy varies: `PullIfNotPresent` (online) or `PullNever` (airgap)

#### Upgrade Orchestration Flow

**File**: `operator/pkg/upgrade/upgrade.go:29-82`

```go
func Upgrade(ctx context.Context, cli client.Client, hcli helm.Client,
    rc runtimeconfig.RuntimeConfig, in *ecv1beta1.Installation) error
```

**Sequence**:
1. **Override installation data dirs** (for version compatibility)
2. **Upgrade k0s** via autopilot
3. **Update cluster config** (images, network settings)
4. **Upgrade addons** (Helm-based)
5. **Upgrade extensions** (Helm-based)
6. **Create host support bundle**
7. Set installation state to `InstallationStateInstalled`

#### Controller Integration

**File**: `operator/controllers/installation_controller.go:386-471`

The InstallationReconciler monitors Installation CRs and:
- Reconciles node statuses
- Copies host preflight results from new nodes
- Cleans up OpenEBS stateful pods
- Updates CA bundle ConfigMap
- **Deletes completed upgrade jobs** when state is `InstallationStateInstalled`

Reconciliation triggers on:
- Installation CR changes
- Node events
- Autopilot Plan changes
- Helm Chart changes

### 2. Artifact Distribution

**File**: `operator/pkg/artifacts/upgrade.go`

#### Artifact Jobs Architecture

For each node, a **copy-artifacts Job** is created:

```go
func EnsureArtifactsJobForNodes(ctx context.Context, cli client.Client,
    rc runtimeconfig.RuntimeConfig, in *ecv1beta1.Installation,
    localArtifactMirrorImage, licenseID, appSlug, channelID, appVersion string) error
```

**Job behavior**:
- **Online mode**: Pulls binaries from replicated.app using local-artifact-mirror CLI
- **Airgap mode**: Pulls binaries, images, and Helm charts from airgap bundle
- Runs on specific node (nodeSelector)
- Mounts host data directory at `/embedded-cluster`

**Commands**:
- **Online** (`operator/pkg/artifacts/upgrade.go:85-94`):
  ```sh
  /usr/local/bin/local-artifact-mirror pull binaries --data-dir /embedded-cluster \
      --app-slug $APP_SLUG --channel-id $CHANNEL_ID --app-version $APP_VERSION $INSTALLATION_DATA
  sleep 10  # Wait for LAM restart
  ```

- **Airgap** (`operator/pkg/artifacts/upgrade.go:96-107`):
  ```sh
  /usr/local/bin/local-artifact-mirror pull binaries --data-dir /embedded-cluster $INSTALLATION_DATA
  /usr/local/bin/local-artifact-mirror pull images --data-dir /embedded-cluster $INSTALLATION_DATA
  /usr/local/bin/local-artifact-mirror pull helmcharts --data-dir /embedded-cluster $INSTALLATION_DATA
  mv /embedded-cluster/bin/k0s /embedded-cluster/bin/k0s-upgrade
  rm /embedded-cluster/images/images-amd64-* || true
  sleep 10  # Wait for LAM restart
  ```

#### Local Artifact Mirror

The **Local Artifact Mirror (LAM)** is a service that:
- Serves artifacts from the host filesystem on `localhost:50000` (default)
- Caches downloaded binaries, images, and Helm charts
- Restarts when it detects EC binary updates
- Acts as a local registry mirror for containerd

**Storage locations** (from RuntimeConfig):
- Binaries: `<data-dir>/bin/`
- Images: `<data-dir>/images/`
- Helm charts: `<data-dir>/charts/`

**Airgap flow**:
1. Artifacts are pulled from airgap bundle to node filesystem
2. K0s can pull images from `http://127.0.0.1:<port>/images/ec-images-amd64.tar`
3. Autopilot uses this URL for airgap upgrades

### 3. K0s Autopilot Integration

**File**: `operator/pkg/upgrade/upgrade.go:96-174`

#### Autopilot Plan Creation

```go
func upgradeK0s(ctx context.Context, cli client.Client, rc runtimeconfig.RuntimeConfig,
    in *ecv1beta1.Installation) error
```

**Flow**:
1. Check if k0s version matches desired version (skip if already upgraded)
2. **Create autopilot upgrade plan** if none exists
3. **Wait for plan completion** (poll every 5 seconds)
4. Verify all nodes match target version
5. Delete successful plan

#### Autopilot Plan Structure

**File**: `operator/pkg/upgrade/installation.go`

```go
func startAutopilotUpgrade(ctx context.Context, cli client.Client, rc runtimeconfig.RuntimeConfig,
    in *ecv1beta1.Installation, meta *ectypes.ReleaseMetadata) error
```

**Plan command** (online):
```go
PlanCommand{
    K0sUpdate: &PlanCommandK0sUpdate{
        Version: desiredK0sVersion,
        Platforms: {
            "linux-amd64": {
                URL: fmt.Sprintf("http://127.0.0.1:%d/...", rc.LocalArtifactMirrorPort()),
            },
        },
        Targets: { /* controller and worker targets */ },
    },
}
```

**Plan command** (airgap):
- Uses `AirgapUpdate` instead of `K0sUpdate`
- Points to local image tarball: `http://127.0.0.1:<port>/images/ec-images-amd64.tar`
- Created via `artifacts.CreateAutopilotAirgapPlanCommand()`

#### Autopilot State Machine

**File**: `operator/pkg/autopilot/autopilot.go`

Plan states:
- `PlanSchedulable`: Being prepared
- `PlanSchedulableWait`: Being prepared
- `PlanCompleted`: Successfully completed
- `PlanWarning`: Failed with warnings
- `PlanInconsistentTargets`: Failed (malformed)
- `PlanApplyFailed`: Apply failed

Helper functions:
- `HasThePlanEnded(plan)`: Check if plan execution finished
- `HasPlanSucceeded(plan)`: Check if plan completed successfully
- `HasPlanFailed(plan)`: Check if plan failed

### 4. Addon Upgrades via Helm

**File**: `pkg/addons/upgrade.go`

#### Addon Upgrade Flow

```go
func (a *AddOns) Upgrade(ctx context.Context, in *ecv1beta1.Installation,
    meta *ectypes.ReleaseMetadata, opts UpgradeOptions) error
```

**Sequence** (`pkg/addons/upgrade.go:56-124`):
1. **OpenEBS** (storage provider)
2. **Embedded Cluster Operator** (reconciliation)
3. **Registry** (if airgap)
4. **SeaweedFS** (if airgap + HA)
5. **Velero** (if disaster recovery enabled)
6. **Admin Console** (KOTS UI)

Each addon:
- Checks Installation condition status (skip if already upgraded)
- Sets condition to `False` with "Upgrading" reason
- Calls addon's `Upgrade()` method
- Sets condition to `True` with "Upgraded" reason on success

#### Addon Upgrade Interface

**File**: `pkg/addons/types/types.go`

```go
type AddOn interface {
    Name() string
    Version() string
    Namespace() string
    ReleaseName() string
    Upgrade(ctx context.Context, logFunc func(string, ...any),
        kcli client.Client, mcli metadata.Interface, hcli helm.Client,
        domains Domains, overrides map[string]interface{}) error
}
```

Each addon implements:
- `Upgrade()`: Helm upgrade logic
- Metadata methods for naming
- Values generation from Installation spec

**Example** (`pkg/addons/embeddedclusteroperator/upgrade.go`):
```go
func (e *EmbeddedClusterOperator) Upgrade(ctx context.Context, ...) error {
    values, err := e.GetValues(...)
    return helmcli.Upgrade(ctx, e.Namespace(), e.ReleaseName(), e.ChartLocation(), values, ...)
}
```

#### Helm Upgrade Options

Helm upgrades use:
- **Atomic**: Rollback on failure
- **Wait**: Wait for resources to be ready
- **Timeout**: Configurable per addon
- **Values**: Generated from Installation CR

### 5. V3 Upgrade CLI Command

**File**: `cmd/installer/cli/upgrade.go`

#### Command Structure

```go
func UpgradeCmd(ctx context.Context, appSlug, appTitle string) *cobra.Command
```

**Flags**:
- `--target`: "linux" or "kubernetes"
- `--license`: Path to license file (required)
- `--airgap-bundle`: Path to airgap bundle
- `--yes`: Assume yes to prompts
- `--manager-port`: Manager UI port

**Flow**:
1. **preRunUpgrade**: Validate and gather upgrade config
   - Get cluster ID from existing Installation
   - Validate data directory exists
   - Read license file
   - Read TLS config from kotsadm-tls secret
   - Read password hash from kotsadm-password secret
   - Get current app config values via KOTS CLI
2. **verifyAndPromptUpgrade**: Verify release and prompt user
3. **runManagerExperienceUpgrade**: Start API and serve manager UI
4. User completes upgrade via web UI

#### Upgrade Config Gathering

**Key differences from install** (`cmd/installer/cli/upgrade.go:229-310`):
- Reads **existing cluster ID** (not generated)
- Reads **existing TLS certificates** from cluster secret
- Reads **existing password hash** from cluster secret
- Reads **existing config values** from KOTS API
- Uses existing **manager port** or user-provided override

```go
type upgradeConfig struct {
    passwordHash       []byte
    tlsConfig          apitypes.TLSConfig
    tlsCert            tls.Certificate
    license            *kotsv1beta1.License
    licenseBytes       []byte
    airgapMetadata     *airgap.AirgapMetadata
    embeddedAssetsSize int64
    configValues       apitypes.AppConfigValues
    endUserConfig      *ecv1beta1.Config
    clusterID          string
    managerPort        int
}
```

### 6. V3 API Architecture and Patterns

#### API Structure

**File**: `api/README.md`

**Key architectural principles**:
1. **Controllers** orchestrate workflows, read from one manager and pass data to another
2. **Managers** handle specific subdomains, remain independent (no cross-dependencies)
3. **State machine** captures workflow state and enforces valid transitions
4. **Store** persists state (in-memory for current implementation)

#### Upgrade Controller

**File**: `api/controllers/linux/upgrade/controller.go`

```go
type UpgradeController struct {
    installationManager installation.InstallationManager
    hostUtils           hostutils.HostUtilsInterface
    netUtils            utils.NetUtils
    releaseData         *release.ReleaseData
    license             []byte
    airgapBundle        string
    configValues        types.AppConfigValues
    clusterID           string
    store               store.Store
    stateMachine        statemachine.Interface
    logger              logrus.FieldLogger
    *appcontroller.AppController  // Composition!
}
```

**Pattern**: Upgrade controller **composes** App controller to reuse app upgrade logic.

#### App Controller

**File**: `api/controllers/app/controller.go`

```go
type AppController struct {
    appConfigManager           appconfig.AppConfigManager
    appInstallManager          appinstallmanager.AppInstallManager
    appPreflightManager        apppreflightmanager.AppPreflightManager
    appReleaseManager          appreleasemanager.AppReleaseManager
    appUpgradeManager          appupgrademanager.AppUpgradeManager  // NEW!
    stateMachine               statemachine.Interface
    // ... other fields
}
```

**App Upgrade Manager** (`api/internal/managers/app/upgrade/`):
- Handles KOTS deploy command execution
- Manages upgrade state (Running → Succeeded/Failed)
- Uses KOTS CLI to sync license, download update, and deploy

#### State Machine

**Upgrade states** (from `proposals/v3_upgrade_workflow.md`):
```
StateNew
  ↓
StateApplicationConfiguring → StateApplicationConfigured
  ↓
StateAppPreflightsRunning → StateAppPreflightsSucceeded/Failed
  ↓
StateAppUpgrading → StateSucceeded/StateAppUpgradeFailed
```

#### API Endpoints

**Linux upgrade endpoints**:
```
POST /api/linux/upgrade/app/upgrade           # Execute app upgrade
GET  /api/linux/upgrade/app/upgrade/status    # Check app upgrade status
GET  /api/linux/upgrade/app/config/template   # Get config template
GET  /api/linux/upgrade/app/config/values     # Get current config
PATCH /api/linux/upgrade/app/config/values    # Update config values
POST /api/linux/upgrade/app-preflights/run    # Run preflights
GET  /api/linux/upgrade/app-preflights/status # Check preflight status
```

Similar endpoints exist for `/api/kubernetes/upgrade/*`.

### 7. V3 Installer Flow Patterns

**File**: `cmd/installer/cli/install.go`

#### Manager Experience Install

```go
func runManagerExperienceInstall(ctx context.Context, flags InstallCmdFlags,
    rc runtimeconfig.RuntimeConfig, ki kubernetesinstallation.Installation,
    metricsReporter metrics.ReporterInterface, appTitle string) error
```

**Flow**:
1. Generate self-signed TLS cert (if not provided)
2. Hash password with bcrypt
3. Create API config with:
   - TLS config
   - Password/hash
   - License
   - Airgap bundle
   - Config values
   - Release data
   - Cluster ID
4. **Start API** in background
5. Print manager URL
6. Wait for context cancellation (user completes via UI)

#### API Startup

**File**: `cmd/installer/cli/api.go` (not shown but referenced)

The API:
- Serves manager UI (React SPA) at `/`
- Serves API endpoints at `/api/*`
- Uses JWT authentication
- Runs state machine for workflow orchestration
- Calls managers for domain logic

**Pattern**: The upgrade command follows the same flow, just with `mode: "upgrade"` instead of `mode: "install"`.

### 8. Integration Points for Infrastructure Upgrade

Based on the research, here are the integration points where infrastructure upgrade would fit:

#### Option 1: Extend V3 Upgrade Command (Recommended)

Add infrastructure upgrade as part of the v3 upgrade wizard:

```
User → Welcome (auth) → Config → Preflights → App Upgrade → Infrastructure Upgrade → Complete
```

**Integration**:
- New state: `StateInfrastructureUpgrading`
- New controller: `LinuxInfrastructureUpgradeController`
- New manager: `InfrastructureUpgradeManager`
- New API endpoints: `/api/linux/upgrade/infrastructure/*`

**Flow**:
1. After app upgrade succeeds, check if infrastructure upgrade needed
2. Display infrastructure upgrade wizard step
3. Execute infrastructure upgrade via operator job (reuse existing logic)
4. Monitor progress via Installation CR status
5. Report completion

#### Option 2: Separate Infrastructure Upgrade Command

Add new command: `<app> upgrade infrastructure`

**Pros**: Clear separation, simpler state machine
**Cons**: Two separate upgrade commands, user confusion

#### Option 3: Hybrid Approach

Detect infrastructure upgrade in app upgrade flow:

```go
func (c *UpgradeController) ShouldUpgradeInfrastructure() bool {
    currentVersion := getCurrentECVersion()
    targetVersion := c.releaseData.ECVersion
    return currentVersion != targetVersion
}
```

If needed, automatically trigger infrastructure upgrade after app upgrade.

### 9. Code Organization

#### Key Packages

**Operator**:
- `operator/pkg/upgrade/`: Upgrade orchestration
  - `upgrade.go`: Main upgrade flow
  - `job.go`: Upgrade job creation
  - `installation.go`: Autopilot plan creation
- `operator/pkg/artifacts/`: Artifact distribution
  - `upgrade.go`: Artifact job management
  - `registry.go`: Registry secret handling
- `operator/pkg/autopilot/`: Autopilot helpers
  - `autopilot.go`: Plan state checking
- `operator/controllers/`: Kubernetes controllers
  - `installation_controller.go`: Installation CR reconciliation

**Addons**:
- `pkg/addons/`: Addon management
  - `upgrade.go`: Addon upgrade orchestration
  - `interface.go`: AddOn interface
  - `<addon>/upgrade.go`: Per-addon upgrade logic
  - `<addon>/values.go`: Helm values generation

**V3 API**:
- `api/controllers/`: API controllers
  - `app/controller.go`: App controller (shared)
  - `linux/upgrade/controller.go`: Linux upgrade controller
  - `kubernetes/upgrade/controller.go`: K8s upgrade controller
- `api/internal/managers/`: Business logic managers
  - `app/upgrade/`: App upgrade manager
  - `app/config/`: Config manager
  - `app/preflight/`: Preflight manager
- `api/internal/statemachine/`: State machine
- `api/types/`: API types

**CLI**:
- `cmd/installer/cli/`: CLI commands
  - `install.go`: Install command
  - `upgrade.go`: Upgrade command
  - `api.go`: API startup
  - `flags.go`: Flag definitions

**Shared**:
- `pkg/kubeutils/`: Kubernetes utilities
- `pkg/helm/`: Helm client wrapper
- `pkg/runtimeconfig/`: Runtime configuration
- `pkg/release/`: Release metadata

#### Directory Structure

```
embedded-cluster/
├── operator/                    # In-cluster operator
│   ├── pkg/
│   │   ├── upgrade/            # Upgrade orchestration
│   │   ├── artifacts/          # Artifact distribution
│   │   └── autopilot/          # Autopilot helpers
│   └── controllers/            # K8s controllers
├── api/                        # V3 API
│   ├── controllers/            # API controllers
│   │   ├── app/               # App controller (shared)
│   │   ├── linux/
│   │   │   ├── install/       # Linux install controller
│   │   │   └── upgrade/       # Linux upgrade controller
│   │   └── kubernetes/
│   │       ├── install/       # K8s install controller
│   │       └── upgrade/       # K8s upgrade controller
│   ├── internal/
│   │   ├── managers/          # Business logic
│   │   │   └── app/
│   │   │       ├── config/    # Config manager
│   │   │       ├── preflight/ # Preflight manager
│   │   │       ├── install/   # Install manager
│   │   │       └── upgrade/   # Upgrade manager
│   │   ├── statemachine/      # State machine
│   │   └── store/             # State persistence
│   └── types/                 # API types
├── cmd/installer/             # CLI binary
│   └── cli/
│       ├── install.go         # Install command
│       └── upgrade.go         # Upgrade command
├── pkg/                       # Shared packages
│   ├── addons/               # Addon management
│   ├── autopilot/            # Autopilot utilities (appears empty)
│   ├── helm/                 # Helm client
│   └── kubeutils/            # K8s utilities
└── web/                      # Manager UI (React)
```

## Architecture Insights

### Upgrade Orchestration Pattern

Embedded Cluster uses a **two-phase upgrade**:

1. **Phase 1: Artifact Distribution** (parallel)
   - Jobs run on each node
   - Pull binaries/images from LAM or airgap bundle
   - No cluster disruption

2. **Phase 2: Upgrade Execution** (sequential)
   - K0s upgrade via autopilot (rolling upgrade per node)
   - Addon upgrades via Helm (one at a time)
   - Extensions upgrade via Helm

### State Machine Pattern

Both install and upgrade use the same state machine pattern:

```go
type Interface interface {
    AcquireLock() (Lock, error)
    Transition(lock Lock, newState State) error
    GetCurrentState() State
}
```

This ensures:
- Only one operation at a time
- Valid state transitions
- Recoverable failures

### Composition vs Inheritance

The upgrade controller **composes** the app controller rather than extending it:

```go
type UpgradeController struct {
    // Upgrade-specific fields
    installationManager installation.InstallationManager
    // ...

    // Composed app controller
    *appcontroller.AppController
}
```

This allows:
- Reusing app upgrade logic
- Adding infrastructure upgrade logic
- Clear separation of concerns

### Airgap Architecture

Airgap installations use:

1. **Local Artifact Mirror**: Serves artifacts from host filesystem
2. **Artifact Jobs**: Copy artifacts from airgap bundle to nodes
3. **Autopilot Airgap Plans**: Pull images from LAM-served tarball
4. **Registry**: In-cluster registry for images
5. **SeaweedFS** (HA): Distributed storage for registry

All components work together to provide a fully offline upgrade experience.

## Historical Context (from proposals/)

**From**: `proposals/v3_upgrade_workflow.md`

Key insights:
- V3 upgrade workflow mirrors v3 install workflow
- Provides manager UI experience for application upgrades
- Maintains KOTS CLI compatibility for smooth kURL migration
- **Does NOT cover infrastructure/EC version upgrades** (future milestone)
- Uses new `kots deploy` command (combines license sync + download + config + deploy)
- Three iterations:
  1. Core upgrade E2E (Welcome → Upgrade → Complete)
  2. Add configuration (Welcome → Config → Upgrade → Complete)
  3. Add preflights (Welcome → Config → Preflights → Upgrade → Complete)

**Key quote**:
> "This milestone focuses exclusively on application upgrades (not infrastructure/EC version changes)"

**Architectural decision**:
> "Instead of using the brittle `kots upstream upgrade` + `kots set-config` approach, we're introducing a new hidden `kots deploy` command that combines license sync, upstream update download, configuration updates, and deployment into a single operation."

## Open Questions

1. **Infrastructure upgrade integration**: Should infrastructure upgrade be:
   - Part of v3 upgrade wizard (before or after app upgrade)?
   - Separate command?
   - Automatic when EC version differs?

2. **Upgrade sequencing**: Should we upgrade:
   - EC first, then app?
   - App first, then EC?
   - Both together?

3. **Rollback**: How do we handle failed infrastructure upgrades?
   - Autopilot doesn't support rollback
   - Need manual recovery process?

4. **Multi-node**: How does infrastructure upgrade work in multi-node?
   - Currently researched single-node only
   - Need to understand HA upgrade flow

5. **Downtime**: Is zero-downtime infrastructure upgrade possible?
   - K0s autopilot does rolling upgrades
   - But addon upgrades might cause brief disruption

6. **Version compatibility**: How do we handle:
   - EC version X with app version Y?
   - Skipping EC versions (X → X+2)?
   - Downgrading?

## Related Research

- `proposals/v3_upgrade_workflow.md` - V3 app upgrade workflow design
- `api/README.md` - V3 API architecture patterns

## Code References

**Operator-based upgrade**:
- `operator/pkg/upgrade/upgrade.go:29-82` - Main upgrade orchestration
- `operator/pkg/upgrade/job.go:39-290` - Upgrade job creation
- `operator/controllers/installation_controller.go:386-471` - Installation reconciliation

**Artifact distribution**:
- `operator/pkg/artifacts/upgrade.go:109-147` - Artifact job creation
- `operator/pkg/artifacts/upgrade.go:85-107` - Artifact job commands

**K0s autopilot**:
- `operator/pkg/upgrade/upgrade.go:96-174` - K0s upgrade via autopilot
- `operator/pkg/autopilot/autopilot.go` - Autopilot helpers

**Addon upgrades**:
- `pkg/addons/upgrade.go:41-159` - Addon upgrade orchestration
- `pkg/addons/embeddedclusteroperator/upgrade.go` - Example addon upgrade

**V3 upgrade CLI**:
- `cmd/installer/cli/upgrade.go:62-163` - Upgrade command definition
- `cmd/installer/cli/upgrade.go:229-310` - Upgrade config gathering
- `cmd/installer/cli/upgrade.go:433-474` - Manager experience upgrade

**V3 API**:
- `api/controllers/linux/upgrade/controller.go` - Linux upgrade controller
- `api/controllers/app/controller.go:39-54` - App controller interface

**V3 install (for comparison)**:
- `cmd/installer/cli/install.go:112-190` - Install command
- `cmd/installer/cli/install.go:642-746` - Manager experience install
