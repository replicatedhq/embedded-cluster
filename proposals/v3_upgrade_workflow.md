# V3 Upgrade Workflow

## TL;DR

We're implementing a v3 upgrade workflow that mirrors the v3 install workflow, providing a manager UI experience for application upgrades while maintaining KOTS CLI compatibility to ensure smooth migration from kURL. This introduces a new "upgrade" command that guides users through a wizard-based upgrade process via the API and manager UI, while the current "update" command remains unchanged and may be repurposed later for binary updates. The `upgrade` command will only be visible in the help text and only run if the `ENABLE_V3` variable is set by the user. This new command will not SET that `ENABLE_V3` variable, but it depends on it being set by the user. A "command not found" error will be shown if the `upgrade` command is run without the `ENABLE_V3` variable set. This milestone focuses exclusively on application upgrades (not infrastructure/EC version changes), reusing existing UI components and API patterns while introducing new upgrade-specific endpoints. Instead of using the brittle `kots upstream upgrade` + `kots set-config` approach, we're introducing a new hidden `kots deploy` command that combines license sync, upstream update download, configuration updates, and deployment into a single operation. This makes the upgrade process more deterministic and reliable. The workflow delivers tangible user value in each iteration: Iteration 1 provides a complete end-to-end upgrade workflow without preflights or app configuration, Iteration 2 adds app configuration management, and Iteration 3 incorporates preflight checks.

## The problem

Currently, the embedded-cluster upgrade process performs in-cluster upgrades, which are inherently brittle and prone to failure. Users run `./installer update` which creates a new version in the admin console, then navigate to upgrade through the admin console. This in-cluster approach lacks the robustness and reliability needed for production environments, while also providing a fragmented user experience that requires understanding multiple tools and interfaces. The new v3 upgrade workflow addresses these limitations by performing out-of-cluster upgrades, which are more robust and reliable.

Additionally, as vendors migrate from kURL to Embedded Cluster v3, they need a smooth transition path that doesn't require them to repackage their applications or change their existing KOTS-based workflows. This is why we're keeping the use of KOTS for application deployment and management instead of the new installer, ensuring vendors can adopt EC v3 without disrupting their existing application packaging and workflows.

## Goals
- Implement a v3 upgrade workflow that provides a guided, UI-based experience for application upgrades
- Maintain KOTS CLI compatibility to ensure smooth migration from kURL without requiring application repackaging

## Non-Goals
- **Cluster/Infrastructure Upgrades**: This milestone focuses exclusively on application upgrades. Upgrading the Embedded Cluster infrastructure, K0s, or other cluster components will be addressed in a future milestone
- **Required Releases**: Handling required releases in the upgrade workflow will be addressed in a separate milestone/proposal
- **Rollback Functionality**: Application rollback functionality will be addressed in a separate milestone/proposal  
- **Consuming Updates**: Advanced update consumption patterns and version management will be addressed in a separate milestone/proposal

## Prototype / design

### Architecture Overview

```
┌───────────────────────────────────────────────────────────┐
│                     Manager UI (React)                    │
│  ┌──────────┬──────────┬────────────┬─────────┬──────────┐│
│  │ Welcome  │  Config  │ Preflights │ Upgrade │ Complete ││
│  │  Step    │   Step   │    Step    │  Step   │  Step    ││
│  └──────────┴──────────┴────────────┴─────────┴──────────┘│
└────────────────────────┬──────────────────────────────────┘
                         │ HTTPS/JWT Auth
┌────────────────────────▼───────────────────────────────────┐
│                         API Layer                          │
│  ┌────────────────────────────────────────────────────┐    │
│  │              Upgrade Routes                        │    │
│  │  /linux/upgrade/*    /kubernetes/upgrade/*         │    │
│  └────────────────────────────────────────────────────┘    │
└────────────────────────┬───────────────────────────────────┘
                         │
┌────────────────────────▼───────────────────────────────────┐
│                   Controller Layer                         │
│  ┌──────────────────────────────────────────────────────┐  │
│  │     Linux/K8s Upgrade Controller (Composite)         │  │
│  │  ┌────────────────────────────────────────────────┐  │  │
│  │  │        App Upgrade Controller                  │  │  │
│  │  │  - Config Manager                              │  │  │
│  │  │  - Preflight Manager                           │  │  │
│  │  │  - Upgrade Manager                             │  │  │
│  │  └────────────────────────────────────────────────┘  │  │
│  └──────────────────────────────────────────────────────┘  │
└────────────────────────┬───────────────────────────────────┘
                         │
┌────────────────────────▼────────────────────────────────────┐
│                  New KOTS Deploy Command                    │
│  - kots deploy --channel-id --channel-sequence              │
│    --config-values --license (optional)                    │
│  - Does: license sync + download + config + deploy       │
└─────────────────────────────────────────────────────────────┘
```

## Implementation Plan

### Iteration Overview

**Iteration 1: Core Upgrade (E2E without preflights or config)**
- Delivers a complete functional upgrade workflow
- User can authenticate, initiate upgrade, and see completion
- Skips configuration and preflight steps entirely
- Focus: Welcome → Upgrade Execution → Completion

**Iteration 2: App Configuration**
- Adds configuration management to the upgrade workflow
- User can review and modify app configuration before upgrade
- Focus: Welcome → Configuration → Upgrade Execution → Completion

**Iteration 3: App Preflights**
- Adds preflight checks before upgrade execution
- User can see preflight results and address issues
- Focus: Welcome → Configuration → Preflights → Upgrade Execution → Completion

Each iteration delivers a complete, functional user experience that can be shipped iteratively.

### Iteration 1: Core Upgrade Implementation

**Deliverable**: Complete end-to-end upgrade workflow (Welcome → Upgrade → Completion)

#### User Flow
```
User → Welcome (auth) → Upgrade (execute) → Complete
         │                     │
         └─────────────────────┘
              API Interactions:
            1. Execute upgrade via KOTS CLI
            2. Monitor progress  
            3. Report completion
```

#### State Machine
```
StateNew
  ↓
StateAppUpgrading → StateSucceeded/StateAppUpgradeFailed
```

#### New Files
```
api/controllers/linux/upgrade/controller.go       # Linux upgrade controller  
api/controllers/kubernetes/upgrade/controller.go  # K8s upgrade controller
api/controllers/app/upgrade.go                    # Upgrade methods for app controller
api/internal/managers/app/upgrade/upgrade.go      # Core upgrade manager
api/internal/managers/app/upgrade/manager.go      # Upgrade manager interface
api/internal/states/upgrade/states.go             # Basic upgrade states
api/internal/store/app/upgrade/store.go           # Upgrade store implementation
api/types/upgrade.go                              # Upgrade-specific types
api/internal/handlers/linux/upgrade.go            # Linux upgrade handlers
api/internal/handlers/kubernetes/upgrade.go       # Kubernetes upgrade handlers
cmd/installer/kotscli/kotscli.go                  # Add KOTS CLI Deploy function
cmd/installer/cli/upgrade.go                      # New upgrade command
```

#### Modified Files
```
api/controllers/app/install/* -> api/controllers/app/*       # Move install controller files up one level
api/controllers/app/controller.go                            # Rename InstallController to AppController, add upgrade interface
api/routes.go                                                # Basic upgrade routes
api/handlers.go                                              # Upgrade handler references
api/internal/handlers/linux/handler.go                       # Add upgrade controller field
api/internal/handlers/kubernetes/handler.go                  # Add upgrade controller field
cmd/installer/cli/flags.go                                   # Upgrade-specific flags
cmd/installer/main.go                                        # Register upgrade command
web/src/types/index.ts                                      # Add 'mode' to InitialState
web/src/providers/InitialStateProvider.tsx                  # Parse mode from window state
web/src/providers/WizardProvider.tsx                        # Step filtering for upgrade
web/src/components/wizard/InstallWizard.tsx                 # Skip config/preflight steps
web/src/components/wizard/installation/InstallationStep.tsx # Endpoint switching
web/src/components/wizard/completion/LinuxCompletionStep.tsx # Upgrade messaging
web/src/components/wizard/completion/KubernetesCompletionStep.tsx # Upgrade messaging
```

#### Tests
```
api/internal/store/app/upgrade/store_test.go                 # Test upgrade state persistence, concurrent access, log truncation
api/internal/managers/app/upgrade/upgrade_test.go            # Test upgrade validation, config merging, KOTS CLI integration
api/controllers/kubernetes/upgrade/controller_test.go        # Test state machine transitions, error handling, metrics reporting
api/controllers/linux/upgrade/controller_test.go             # Test state machine transitions, error handling, metrics reporting
api/controllers/app/controller_test.go                       # Test app upgrade controller with mocked dependencies (update existing)
api/integration/kubernetes/upgrade/appupgrade_test.go        # Test full HTTP request/response cycles with real API instances
api/integration/linux/upgrade/appupgrade_test.go             # Test Linux upgrade API endpoints end-to-end
api/integration/app/upgrade_test.go                          # Test app upgrade API integration (moved from install subdirectory)
web/src/components/wizard/upgrade/tests/InstallWizard.test.tsx # Update to test upgrade wizard flow
web/src/components/wizard/upgrade/tests/InstallationStep.test.tsx # Update to test upgrade endpoint switching
```

#### API Endpoints
```
POST /api/linux/upgrade/app/upgrade        # Execute app upgrade
GET  /api/linux/upgrade/app/upgrade/status # Check app upgrade status
POST /api/kubernetes/upgrade/app/upgrade   # Execute app upgrade (K8s)
GET  /api/kubernetes/upgrade/app/upgrade/status # Check app upgrade status (K8s)
```

#### Sub-Iterations

**1.1 New KOTS Deploy Command**
**1.2 API Changes**
**1.3 New Upgrade CLI Command** 
**1.4 Frontend Changes**

#### Pseudo Code

**1.1 New KOTS Deploy Command:**

```go
// cmd/kots/cli/deploy.go
type DeployOptions struct {
    AppSlug         string
    ChannelID       string
    ChannelSequence int64
    ConfigValues    string
    License         string
    AirgapBundle    string
    SkipPreflights  bool
}

func DeployCmd() *cobra.Command {
    // Pseudo code: Create hidden deploy command
    //
    // Command: kots deploy [appSlug] (hidden from help)
    // Flags:
    //   --config-values <path> (required): Path to config values file
    //   --license <path> (optional): License file path for airgap
    //   --channel-id <string> (optional): Channel ID for online deployment
    //   --channel-sequence <int> (optional): Channel sequence for online deployment  
    //   --airgap-bundle <path> (optional): Path to airgap bundle
    //   --skip-preflights (bool): Skip preflight checks
    //
    // Validation:
    // - Either (channel-id AND channel-sequence) OR airgap-bundle required
    // - License only valid with airgap-bundle
    // - Config-values file must exist and be valid
}

func runDeploy(v *viper.Viper, args []string) error {
    // Pseudo code: Deploy execution flow
    //
    // 1. Parse and validate flags
    // 2. Establish port-forward to kotsadm (similar to set-config pattern)
    // 3. Sequential API calls via HTTP:
    //    a. Sync license (if provided) - POST /api/v1/app/{appSlug}/license
    //    b. Upstream update - POST /api/v1/app/{appSlug}/upstream/update
    //    c. Set config and deploy - POST /api/v1/app/{appSlug}/config/values
    // 4. Monitor deployment progress
    // 5. Return success/failure
}
```

New API Handler:
```go
// pkg/handlers/update.go
type UpstreamUpdateResponse struct {
    Success bool   `json:"success"`
    Error   string `json:"error,omitempty"`
}

func (h *Handler) UpstreamUpdate(w http.ResponseWriter, r *http.Request) {
    // Pseudo code: Upstream update handler (online and airgap)
    // Only used by V3 embedded cluster installs
    //
    // 1. Parse URL query parameters:
    //    - channelId: Channel ID for online updates
    //    - channelSequence: Channel sequence for online updates  
    //    - skipPreflights: Boolean flag to skip preflights
    // 2. Get app from appSlug parameter
    // 3. Check Content-Type header:
    //    - If "multipart/form-data": Handle airgap update (same as upstream upgrade handler)
    //    - Else: Handle online update
    // 4. For online updates:
    //    - Create upstreamtypes.Update with ChannelID, Cursor, AppSequence: nil
    //    - Call upstream.DownloadUpdate to fetch and create new version
    // 5. Return UpstreamUpdateResponse with success/error
}
```

Route Registration:
```go
// pkg/handlers/handlers.go
func RegisterRoutes(handler *Handler, kotsadmRouter *mux.Router) {
    // ... existing routes
    
    // New route for deploy command
    kotsadmRouter.Path("/api/v1/app/{appSlug}/upstream/update").Methods("POST").
        HandlerFunc(middleware.EnforceAccess(rbac.AppWrite, handler.UpstreamUpdate))
}
```

**1.2 API Changes:**

App Upgrade Manager Interface:
```go
// api/internal/managers/app/upgrade/manager.go
type AppUpgradeManager interface {
    // Upgrade upgrades the app with the provided config values (matching install manager pattern)
    Upgrade(ctx context.Context, configValues kotsv1beta1.ConfigValues) error
    // GetStatus returns the current app upgrade status
    GetStatus() (types.AppUpgrade, error)
}
```

App Upgrade Manager Implementation:
```go
// api/internal/managers/app/upgrade/upgrade.go
type appUpgradeManager struct {
    appUpgradeStore appupgradestore.Store
    releaseData     *release.ReleaseData
    license         []byte
    clusterID       string
    airgapBundle    string
    kotsCLI         KotsCLIDeployer
    logger          logrus.FieldLogger
}

// Upgrade upgrades the app with the provided config values
func (m *appUpgradeManager) Upgrade(ctx context.Context, configValues kotsv1beta1.ConfigValues) (finalErr error) {
    // Pseudo code: Upgrade workflow
    //
    // 1. Set status to StateRunning with "Upgrading application" message
    // 2. Defer function to handle panic recovery and final status setting:
    //    - On panic: set status to StateFailed with panic details
    //    - On error: set status to StateFailed with error message
    //    - On success: set status to StateSucceeded with "Upgrade complete" message
    // 3. Call m.upgrade(ctx, configValues) to perform actual upgrade
    // 4. Return error if upgrade fails, nil on success
}

func (m *appUpgradeManager) upgrade(ctx context.Context, configValues kotsv1beta1.ConfigValues) error {
    license := &kotsv1beta1.License{}
    if err := kyaml.Unmarshal(m.license, license); err != nil {
        return fmt.Errorf("parse license: %w", err)
    }

    ecDomains := utils.GetDomains(m.releaseData)

    deployOpts := kotscli.DeployOptions{
        AppSlug:      license.Spec.AppSlug,
        License:      m.license,
        Namespace:    constants.KotsadmNamespace,
        ClusterID:    m.clusterID,
        AirgapBundle: m.airgapBundle,
        ChannelID:    m.releaseData.ChannelID,
        ChannelSequence: m.releaseData.ChannelSequence,
        // Skip running the KOTS app preflights in the Admin Console; they run in the manager experience installer when ENABLE_V3 is enabled
        SkipPreflights:        os.Getenv("ENABLE_V3") == "1",
        ReplicatedAppEndpoint: netutils.MaybeAddHTTPS(ecDomains.ReplicatedAppDomain),
        Stdout:                m.newLogWriter(),
    }

    configValuesFile, err := m.createConfigValuesFile(configValues)
    if err != nil {
        return fmt.Errorf("creating config values file: %w", err)
    }
    deployOpts.ConfigValuesFile = configValuesFile

    if m.kotsCLI != nil {
        return m.kotsCLI.Deploy(deployOpts)
    }

    return kotscli.Deploy(deployOpts)
}
```

Upgrade Types:
```go
// api/types/upgrade.go
type AppUpgrade struct {
    Status types.Status `json:"status"`
}

type UpgradeAppRequest struct {
    IgnoreAppPreflights bool `json:"ignoreAppPreflights"`
}
```

KOTS CLI Deploy Options:
```go
// cmd/installer/kotscli/kotscli.go
type DeployOptions struct {
    AppSlug               string
    License               []byte
    Namespace             string
    ClusterID             string
    AirgapBundle          string
    ConfigValuesFile      string
    ChannelID             string
    ChannelSequence       int64
    ReplicatedAppEndpoint string
    SkipPreflights        bool
    Stdout                io.Writer
}

func Deploy(opts DeployOptions) error {
    // Pseudo code: Execute new kots deploy command
    // 
    // Command: kots deploy <app-slug>
    // Flags:
    //   --license <temp-license-file> (optional - if not provided, syncs current license)
    //   --channel-id <opts.ChannelID> (from release metadata)
    //   --channel-sequence <opts.ChannelSequence> (from release metadata)
    //   --config-values <opts.ConfigValuesFile> (if provided)
    //   --skip-preflights (if opts.SkipPreflights is true)
    //
    // Environment variables:
    //   EMBEDDED_CLUSTER_ID=<opts.ClusterID>
    //   REPLICATED_APP_ENDPOINT=<opts.ReplicatedAppEndpoint> (if provided)
    //
    // This single command performs:
    // 1. License sync (with optional new license data)
    // 2. Download upstream update for specific channel/sequence
    // 3. Update configuration with provided values
    // 4. Deploy the release
    //
    // Output: Use opts.Stdout for command output with masking
    //
    // Return: error if command fails, nil on success
}
```

Upgrade Store:
```go
// api/internal/store/app/upgrade/store.go
type Store interface {
    SetStatus(status types.Status) error
    GetStatus() (types.AppUpgrade, error)
}

type memoryStore struct {
    status types.Status
    mutex  sync.RWMutex
}

func NewMemoryStore() Store {
    return &memoryStore{
        status: types.Status{
            State:       types.StateNew,
            Description: "",
            LastUpdated: time.Now(),
        },
    }
}

func (s *memoryStore) SetStatus(status types.Status) error {
    s.mutex.Lock()
    defer s.mutex.Unlock()
    s.status = status
    return nil
}

func (s *memoryStore) GetStatus() (types.AppUpgrade, error) {
    s.mutex.RLock()
    defer s.mutex.RUnlock()
    return types.AppUpgrade{Status: s.status}, nil
}
```

App Controller Extension:
```go
// api/controllers/app/controller.go - Iteration 1 changes
// Rename InstallController to AppController
type AppController struct {
    appConfigManager           appconfig.AppConfigManager
    appInstallManager          appinstallmanager.AppInstallManager
    appPreflightManager        apppreflightmanager.AppPreflightManager
    appReleaseManager          appreleasemanager.AppReleaseManager
    appUpgradeManager          appupgrademanager.AppUpgradeManager // NEW
    stateMachine               statemachine.Interface
    // ... other existing fields
}

// Extend Controller interface to include upgrade methods
type Controller interface {
    // existing methods ...
    
    // NEW: Upgrade methods
    UpgradeApp(ctx context.Context, ignoreAppPreflights bool) error
    GetAppUpgradeStatus(ctx context.Context) (types.AppUpgrade, error)
}
```

Upgrade Implementation:
```go
// api/controllers/app/upgrade.go - Iteration 1
func (c *AppController) UpgradeApp(ctx context.Context, ignoreAppPreflights bool) error {
    // Same pattern as InstallApp but with upgrade states and upgrade manager
    lock, err := c.stateMachine.AcquireLock()
    if err != nil {
        return types.NewConflictError(err)
    }
    defer lock.Release()

    // Transition to StateAppUpgrading (upgrade states)
    err = c.stateMachine.Transition(lock, states.StateAppUpgrading)
    if err != nil {
        return fmt.Errorf("transition states: %w", err)
    }

    // Get config values for app upgrade (same as install pattern)
    configValues, err := c.appConfigManager.GetKotsadmConfigValues()
    if err != nil {
        return fmt.Errorf("get kotsadm config values for app upgrade: %w", err)
    }

    // Call upgrade manager with config values (matching install manager pattern)
    err = c.appUpgradeManager.Upgrade(ctx, configValues)
    if err != nil {
        return fmt.Errorf("upgrade app: %w", err)
    }

    return nil
}

func (c *AppController) GetAppUpgradeStatus(ctx context.Context) (types.AppUpgrade, error) {
    // Same pattern as GetAppInstallStatus but for upgrade
    return c.appUpgradeManager.GetStatus()
}
```

Linux/K8s Upgrade Controllers:
```go
// api/controllers/linux/upgrade/controller.go
type LinuxUpgradeController struct {
    *app.Controller // Composition
    store Store
}

func NewLinuxUpgradeController(opts) *LinuxUpgradeController {
    // Initialize app upgrade controller
    // Setup Linux-specific configuration
    // Return composite controller
}
```

Do the same for the Kubernetes target.

Handler Functions:
```go
// api/internal/handlers/linux/upgrade.go
func (h *Handler) PostUpgradeApp(w http.ResponseWriter, r *http.Request) {
    var req types.UpgradeAppRequest
    if err := utils.BindJSON(w, r, &req, h.logger); err != nil {
        return
    }

    err := h.upgradeController.UpgradeApp(r.Context(), req.IgnoreAppPreflights)
    if err != nil {
        utils.LogError(r, err, h.logger, "failed to upgrade app")
        utils.JSONError(w, r, err, h.logger)
        return
    }

    h.GetAppUpgradeStatus(w, r)
}

func (h *Handler) GetAppUpgradeStatus(w http.ResponseWriter, r *http.Request) {
    appUpgrade, err := h.upgradeController.GetAppUpgradeStatus(r.Context())
    if err != nil {
        utils.LogError(r, err, h.logger, "failed to get app upgrade status")
        utils.JSONError(w, r, err, h.logger)
        return
    }

    utils.JSON(w, r, http.StatusOK, appUpgrade, h.logger)
}
```

Do the same for the Kubernetes target.

API Routes:
```go
// api/routes.go additions
func (a *API) registerLinuxUpgradeRoutes(router *mux.Router) {
    upgradeRouter := router.PathPrefix("/upgrade").Subrouter()
    
    upgradeRouter.HandleFunc("/app/upgrade", a.handlers.linux.PostUpgradeApp).Methods("POST")
    upgradeRouter.HandleFunc("/app/upgrade/status", a.handlers.linux.GetAppUpgradeStatus).Methods("GET")
}
```

Do the same for the Kubernetes target.

**1.3 New Upgrade CLI Command**
```go
// cmd/installer/cli/upgrade.go
func UpgradeCmd(ctx context.Context, appSlug, appTitle string) *cobra.Command {
	// Pseudo code: Create upgrade command similar to InstallCmd but:
	// 1. Set command name to "upgrade"
	// 2. Always set ENABLE_V3=1 in preRun
	// 3. Validate existing installation exists (opposite of install)
	// 4. Detect existing cluster ID instead of generating new one
	// 5. Use existing data directory instead of creating new one
	// 6. Call runManagerExperienceUpgrade instead of runManagerExperienceInstall
	// 7. Use upgrade metrics reporter instead of install metrics reporter
  // 8. Read TLS certificates to use for the manager UI from the kotsadm-tls secret in the cluster
}
```

**1.4 Frontend Changes**
```typescript
// web/src/types/index.ts - Add mode to initial state
interface InitialState {
  title: string;
  icon?: string;
  installTarget: InstallationTarget;
  mode: "install" | "upgrade"; // NEW: Mode detection
}

// web/src/providers/InitialStateProvider.tsx - Parse mode from server
export const InitialStateProvider: React.FC<{ children: React.ReactNode }> = ({
  children,
}) => {
  const initialState: Partial<InitialState> = window.__INITIAL_STATE__ || {};
  
  const state = {
    title: initialState.title || "My App",
    icon: initialState.icon,
    installTarget: parseInstallationTarget(initialState.installTarget || "linux"),
    mode: initialState.mode || "install", // NEW: Parse mode from server
  };
  
  return (
    <InitialStateContext.Provider value={state}>
      {children}
    </InitialStateContext.Provider>
  );
};

// web/src/components/wizard/InstallWizard.tsx - Step filtering
const InstallWizard: React.FC = () => {
  const { mode, target } = useWizard();
  
  const getSteps = (): WizardStep[] => {
    if (mode === "upgrade") {
      // ITERATION 1: Skip config and preflights for upgrade
      if (target === "kubernetes") {
        return ["welcome", "installation", "kubernetes-completion"];
      } else {
        return ["welcome", "installation", "linux-completion"];
      }
    }
    // Existing install steps (unchanged)
    if (target === "kubernetes") {
      return ["welcome", "configuration", "kubernetes-setup", "installation", "kubernetes-completion"];
    } else {
      return ["welcome", "configuration", "linux-setup", "installation", "linux-completion"];
    }
  };
};

// web/src/components/wizard/installation/InstallationStep.tsx - Endpoint switching
const InstallationStep: React.FC = () => {
  const { mode, target, text } = useWizard();
  const apiBase = `/api/${target}/${mode}`; // e.g., /api/linux/upgrade
  
  const checkStatus = async () => {
    const response = await fetch(`${apiBase}/app/${mode}/status`); // /app/upgrade/status
    // ... handle response
  };
  
  const executeAction = async () => {
    const response = await fetch(`${apiBase}/app/${mode}`, {
      method: 'POST',
      headers: getAuthHeaders(),
    });
    // ... handle response
  };
};
```

### Iteration 2: App Configuration

**Deliverable**: Configuration management in upgrade workflow

#### User Flow
```
User → Welcome (auth) → Config (review/update) → Upgrade (execute) → Complete
         │                  │                        │
         └──────────────────┴────────────────────────┘
                        API Interactions:
                    1. Get current config
                    2. Update config values
                    3. Execute upgrade via KOTS CLI  
                    4. Monitor progress
                    5. Report completion
```

#### State Machine
```
StateNew
  ↓
StateApplicationConfiguring → StateApplicationConfigured
  ↓
StateAppUpgrading → StateSucceeded/StateAppUpgradeFailed
```

#### Modified Files
```
api/controllers/app/controller.go                 # Add config interface methods (already implemented)
api/routes.go                                      # Add config routes
web/src/components/wizard/InstallWizard.tsx       # Include config step (remove skip)
web/src/components/wizard/config/ConfigurationStep.tsx # Endpoint switching logic
```

#### Tests
```
api/integration/kubernetes/upgrade/appconfig_test.go        # Test Kubernetes upgrade app configuration API integration
api/integration/linux/upgrade/appconfig_test.go             # Test Linux upgrade app configuration API integration
web/src/components/wizard/upgrade/tests/ConfigurationStep.test.tsx # Update to test upgrade endpoint switching
```

#### API Endpoints
```
GET   /api/linux/upgrade/app/config/template      # Get config template
GET   /api/linux/upgrade/app/config/values        # Get current config
PATCH /api/linux/upgrade/app/config/values        # Update config values
```

#### Pseudo Code

**2.1 API Changes:**

API Routes Extension:
```go
// api/routes.go additions
func (a *API) registerLinuxUpgradeRoutes(router *mux.Router) {
    upgradeRouter := router.PathPrefix("/upgrade").Subrouter()
    
    upgradeRouter.HandleFunc("/app/config/template", a.handlers.linux.PostTemplateAppConfig).Methods("POST")
    upgradeRouter.HandleFunc("/app/config/values", a.handlers.linux.GetAppConfigValues).Methods("GET")
    upgradeRouter.HandleFunc("/app/config/values", a.handlers.linux.PatchAppConfigValues).Methods("PATCH")
    // ... existing routes
}
```

**2.2 Frontend Changes**
```typescript
// web/src/components/wizard/InstallWizard.tsx - Include config step
const getSteps = (): WizardStep[] => {
  if (mode === "upgrade") {
    // ITERATION 2: Include config step, still skip preflights
    if (target === "kubernetes") {
      return ["welcome", "configuration", "installation", "kubernetes-completion"];
    } else {
      return ["welcome", "configuration", "installation", "linux-completion"];
    }
  }
  // ... existing install steps unchanged
};

// web/src/components/wizard/config/ConfigurationStep.tsx - Endpoint switching
const ConfigurationStep: React.FC<Props> = ({ onNext }) => {
  const { mode, target } = useWizard();
  
  // Construct API path based on mode (install vs upgrade)
  const apiBasePath = `/api/${target}/${mode}`; // e.g., /api/linux/upgrade
  
  const fetchConfig = async () => {
    // Existing fetch logic, just different endpoint
    const response = await fetch(`${apiBasePath}/app/config/values`, {
      headers: getAuthHeaders(),
    });
    // ... rest unchanged
  };
  
  const saveConfig = async (values: AppConfigValues) => {
    const response = await fetch(`${apiBasePath}/app/config/values`, {
      method: 'PATCH',
      headers: getAuthHeaders(),
      body: JSON.stringify(values),
    });
    // ... rest unchanged
  };
  
  // Component render logic remains exactly the same
  return (
    <div className="space-y-6">
      {/* Existing UI unchanged */}
    </div>
  );
};
```

### Iteration 3: App Preflights

**Deliverable**: Preflight checks in upgrade workflow

#### User Flow
```
User → Welcome (auth) → Config (review/update) → Preflights → Upgrade (execute) → Complete
         │                  │                        │              │
         └──────────────────┴────────────────────────┴──────────────┘
                                    API Interactions:
                                1. Get current config
                                2. Update config values
                                3. Run preflights
                                4. Execute upgrade via KOTS CLI
                                5. Monitor progress
                                6. Report completion
```

#### State Machine
```
StateNew
  ↓
StateApplicationConfiguring → StateApplicationConfigured
  ↓
StateAppPreflightsRunning → StateAppPreflightsSucceeded/Failed
  ↓
StateAppUpgrading → StateSucceeded/StateAppUpgradeFailed
```

#### Modified Files
```
api/controllers/app/controller.go                 # Add preflight interface methods (already implemented)
api/internal/managers/app/upgrade/upgrade.go      # Preflight integration
api/internal/states/upgrade/states.go             # Preflight states
api/routes.go                                      # Add preflight routes
web/src/components/wizard/InstallWizard.tsx       # Include preflight step (remove skip)
web/src/components/wizard/preflights/PreflightsStep.tsx # Endpoint switching logic (if needed)
```

#### Tests
```
api/integration/kubernetes/upgrade/apppreflight_test.go      # Test Kubernetes upgrade app preflight API integration
api/integration/linux/upgrade/apppreflight_test.go           # Test Linux upgrade app preflight API integration
web/src/components/wizard/upgrade/tests/PreflightsStep.test.tsx # Update to test upgrade endpoint switching
```

#### API Endpoints
```
POST /api/linux/upgrade/app-preflights/run        # Run preflights
GET  /api/linux/upgrade/app-preflights/status     # Check preflight status
GET  /api/linux/upgrade/app-preflights/output     # Get preflight output
GET  /api/linux/upgrade/app-preflights/titles     # Get preflight titles
```

#### Pseudo Code

**3.1 API Changes:**

API Routes Extension:
```go
// api/routes.go additions
func (a *API) registerLinuxUpgradeRoutes(router *mux.Router) {
    upgradeRouter := router.PathPrefix("/upgrade").Subrouter()
    
    // ... existing routes
    upgradeRouter.HandleFunc("/app-preflights/run", a.handlers.linux.PostRunAppPreflights).Methods("POST") // NEW
    upgradeRouter.HandleFunc("/app-preflights/status", a.handlers.linux.GetAppPreflightsStatus).Methods("GET") // NEW
    // ... existing routes
}
```

**3.2 Frontend Changes**
```typescript
// web/src/components/wizard/InstallWizard.tsx - Include all steps
const getSteps = (): WizardStep[] => {
  if (mode === "upgrade") {
    // ITERATION 3: Full upgrade flow with preflights
    if (target === "kubernetes") {
      return ["welcome", "configuration", "preflights", "installation", "kubernetes-completion"];
    } else {
      return ["welcome", "configuration", "preflights", "installation", "linux-completion"];
    }
  }
  // ... existing install steps unchanged
};

// web/src/components/wizard/preflights/PreflightsStep.tsx - Endpoint switching (if needed)
const PreflightsStep: React.FC = () => {
  const { mode, target } = useWizard();
  const apiBase = `/api/${target}/${mode}`; // e.g., /api/linux/upgrade
  
  const runPreflights = async () => {
    const response = await fetch(`${apiBase}/app-preflights/run`, {
      method: 'POST',
      headers: getAuthHeaders(),
    });
    // ... rest unchanged
  };
  
  const checkPreflightStatus = async () => {
    const response = await fetch(`${apiBase}/app-preflights/status`);
    // ... rest unchanged
  };
  
  // Component render logic remains exactly the same
};
```

## New Subagents / Commands

No new subagents or commands will be created. The implementation reuses existing patterns and components.

## Database

**No database changes required.**

The upgrade workflow uses the same storage patterns as installation, leveraging existing stores:
- `AppConfigStore` - Already handles configuration persistence
- `AppPreflightStore` - Already handles preflight results
- State machine uses in-memory state management


### Command Strategy

**New "upgrade" command**: Provides the v3 upgrade workflow via manager UI
- Sets `ENABLE_V3=1` internally to indicate v3 mode
- No feature flag toggling required - command is always available
- No entitlement required (upgrade available to all users with valid license)

**Existing "update" command**: Remains unchanged
- Current CLI-based update functionality preserved
- May be repurposed later for in-place binary updates
- Users can choose between `upgrade` (UI) or `update` (CLI) workflows

### External contracts

**APIs consumed:**
- New KOTS CLI (`kots deploy`) - Performs license sync, download, config, and deployment
- Existing embedded-cluster API for cluster state

**Events emitted:**
- Upgrade metrics to telemetry service


## Backward compatibility

### API versioning
- New `/upgrade/` endpoints separate from `/install/`
- Existing CLI upgrade path remains functional

### Data format compatibility
- Configuration format unchanged
- State machine states compatible with existing monitoring
- Metrics use same format as install metrics

### Migration windows
- New "upgrade" command available immediately after deployment
- Users can choose between "upgrade" (UI) or "update" (CLI) workflows  
- No forced migration required

## Migrations

**No special deployment handling required.**

The upgrade workflow is additive and doesn't modify existing installation or update flows. Deployment involves:
1. Deploy new API endpoints (backward compatible)
2. Deploy updated UI bundle (detects mode from server)
3. Deploy new "upgrade" command (sets `ENABLE_V3=1` internally)

## Trade-offs

**Optimizing for:** User experience and visual feedback during upgrades

**Trade-off choices:**
1. **Component reuse over custom upgrade UI** - Faster development, consistent UX, but less upgrade-specific optimizations
2. **Single wizard flow over separate upgrade app** - Simpler maintenance, unified codebase, but larger bundle size
3. **Synchronous upgrade over background processing** - Simpler state management, immediate feedback, but blocks UI during upgrade
4. **Reuse install state machine over custom upgrade states** - Less code duplication, but some unused states in memory

## Alternative solutions considered

1. **Separate upgrade application**
   - Rejected: Would duplicate significant code, increase maintenance burden

2. **CLI-only with enhanced output**
   - Rejected: Doesn't meet user expectations for guided experience

3. **Minimal UI wrapper around CLI**
   - Rejected: Inconsistent with v3 install experience

4. **Background upgrade with notification system**
   - Rejected: Users want to monitor upgrade progress actively

5. **Custom upgrade components**
   - Rejected: Existing components work well with endpoint switching

## Research

### Prior art in our codebase
Reference: [V3 Upgrade Workflow Research](./v3_upgrade_workflow_research.md)

**Key findings:**
- Existing v3 install workflow provides complete component set
- State machine pattern proven for multi-step workflows
- Authentication and session management already robust
- Progress tracking infrastructure mature

### License synchronization
The new `kots deploy` command handles license synchronization as its first step, either with a provided license file or by syncing the current license. This eliminates the need for separate license management logic in our upgrade workflow. The new command:
- Optionally accepts a new license file via `--license` flag for airgap mode
- If no license provided, syncs the current license from the cluster
- Downloads the update for the specific channel/sequence provided
- Updates configuration and deploys in a single operation

This approach means the upgrade process is more deterministic and handles edge cases (like channel changes or required releases) more gracefully than the previous brittle two-step process.

### External references
- KOTS upgrade documentation: Details on `kubectl-kots upstream upgrade` command
- React Router v6: For conditional routing based on mode
- Material-UI: Existing component library patterns

### Spikes conducted
- Tested endpoint switching in ConfigurationStep - works without component changes
- Verified `kots upstream upgrade` license sync behavior - confirmed automatic
- Validated state machine can skip states - no issues found

## Checkpoints (PR plan)

### Iteration 1 PR: Core Upgrade E2E
**Deliverable**: Working upgrade flow (Welcome → Upgrade → Complete)
1. **1.1 New KOTS Deploy Command**: Hidden command with channel-based deployment
2. **1.2 API Changes**: Basic upgrade controller and manager, upgrade API endpoints
3. **1.3 New Upgrade CLI Command**: New upgrade command and CLI entry point  
4. **1.4 Frontend Changes**: Mode detection and step filtering (skip config/preflights)
5. Upgrade execution via new KOTS deploy CLI
6. Basic state machine (StateNew → StateAppUpgrading → Success/Failed)
7. Tests for core upgrade functionality

### Iteration 2 PR: Add Configuration
**Deliverable**: Configuration management in upgrade flow
1. Configuration API endpoints (/app/config/template, /app/config/values)
2. Configuration state machine states
3. Frontend configuration step inclusion
4. KOTS set config integration after upgrade
5. Tests for configuration workflow

### Iteration 3 PR: Add Preflights
**Deliverable**: Preflight checks in upgrade flow  
1. Preflight API endpoints (/app-preflights/run, /app-preflights/status)
2. Preflight state machine states
3. Frontend preflight step inclusion
4. Preflight execution and result handling
5. Tests for preflight workflow

Each PR delivers a complete, functional user experience that can be shipped iteratively.