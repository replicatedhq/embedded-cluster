# kURL Migration API Foundation

## TL;DR (solution in one paragraph)

Build REST API endpoints to enable Admin Console UI integration for migrating single-node kURL clusters to Embedded Cluster V3. The API provides three core endpoints: GET /api/linux/kurl-migration/config for configuration discovery, POST /api/linux/kurl-migration/start for initiating async kURL migration, and GET /api/linux/kurl-migration/status for progress monitoring. This foundation enables the UI to guide users through kURL migration while the backend orchestrates the complex multi-phase process of transitioning from kURL to EC without data loss.

## The problem

kURL users need a path to migrate to Embedded Cluster V3, but the kURL migration process is complex, requiring careful orchestration of configuration extraction, network planning, data transfer, and service transitions. Without API endpoints, there's no way for the Admin Console UI to provide a guided kURL migration experience. Users are affected by the inability to modernize their infrastructure, and we know from customer feedback that manual kURL migration attempts have resulted in data loss and extended downtime. Metrics show 40% of kURL installations are candidates for migration.

## Prototype / design

### API Flow Diagram
```
┌─────────────┐     GET /config       ┌─────────────┐
│   Admin     │ ◄──────────────────►  │kURL Migration│
│   Console   │                       │     API     │
│     UI      │  POST /start ────────►│             │
│             │                       │  ┌────────┐ │
│             │  GET /status ────────►│  │Backend │ │
└─────────────┘      (polling)        │  │Process │ │
                                      └──┴────────┴─┘
                                             │
                                             ▼
                                      ┌─────────────┐
                                      │   5-Phase   │
                                      │Orchestration│
                                      └─────────────┘
```

### Data Flow
1. UI requests config → API extracts kURL config, merges with EC defaults
2. UI posts user preferences → API validates, generates kURL migration ID, starts async process
3. UI polls status → API returns current kURL migration phase, progress, messages
4. Background process executes phases: Discovery → Preparation → ECInstall → DataTransfer → Completed

### Key Interfaces
```go
type Controller interface {
    GetInstallationConfig(ctx) (LinuxInstallationConfigResponse, error)
    StartKURLMigration(ctx, transferMode, config) (migrationID string, error)
    GetKURLMigrationStatus(ctx) (KURLMigrationStatusResponse, error)
    Run(ctx) error
}

type Manager interface {
    // GetKurlConfig extracts config from kURL cluster and calculates non-overlapping CIDRs
    // Returns EC-ready LinuxInstallationConfig with NEW CIDRs that don't conflict with kURL
    GetKurlConfig(ctx) (LinuxInstallationConfig, error)

    GetECDefaults(ctx) (LinuxInstallationConfig, error)
    MergeConfigs(user, kurl, defaults) LinuxInstallationConfig
    ValidateTransferMode(mode) error
    ExecutePhase(ctx, phase) error
}

// Internal helper (not exposed in interface)
func CalculateNonOverlappingCIDRs(kurlPodCIDR, kurlServiceCIDR, globalCIDR string) (podCIDR, serviceCIDR string, error)

type Store interface {
    InitializeMigration(id, mode, config) error
    GetMigrationID() (string, error)
    GetStatus() (KURLMigrationStatusResponse, error)
    GetUserConfig() (LinuxInstallationConfig, error)
    SetUserConfig(config) error
    SetState(state) error
    SetPhase(phase) error
    SetError(errorMsg) error
}
```

## New Subagents / Commands

No new subagents or commands will be created in this PR. The API foundation provides programmatic access only.

## Database

**No database changes.** This PR uses an in-memory store. The persistent file-based store will be implemented in PR 7 (sc-130972).

## Implementation plan

### Pseudo Code - Key Components

**Already Implemented (review/enhance):**

**`api/routes.go`** - Route registration
```go
// Register kURL migration routes under /api/linux/kurl-migration/ with auth middleware
// GET  /config - Get installation configuration
// POST /start  - Start kURL migration with transfer mode and optional config
// GET  /status - Poll kURL migration status
```

**`api/handlers.go`** - Handler initialization
```go
// Add KURLMigration field to LinuxHandlers struct
// Initialize kURL migration store, manager, controller in NewLinuxHandlers()
// Wire up dependencies: store -> manager -> controller -> handler
```

**`api/internal/handlers/kurlmigration/handler.go`** - HTTP handlers with Swagger docs
```go
type Handler struct {
    logger                      *logrus.Logger
    kurlMigrationController     Controller
}

func NewHandler(controller Controller, logger *logrus.Logger) *Handler

// GetInstallationConfig returns kURL config merged with EC defaults (values/defaults/resolved)
// @Router /api/linux/kurl-migration/config [get]
func (h *Handler) GetInstallationConfig(w http.ResponseWriter, r *http.Request) {
    // Call controller.GetInstallationConfig(r.Context())
    // Use utils.JSON() to return LinuxInstallationConfigResponse with 200
    // Use utils.JSONError() to handle errors (controller returns typed errors)
}

// PostStartKURLMigration initiates kURL migration with transfer mode and optional config overrides
// @Router /api/linux/kurl-migration/start [post]
func (h *Handler) PostStartKURLMigration(w http.ResponseWriter, r *http.Request) {
    // Use utils.BindJSON() to parse StartKURLMigrationRequest body
    // Call controller.StartKURLMigration(r.Context(), req.TransferMode, req.Config)
    // Use utils.JSON() to return StartKURLMigrationResponse with 200
    // Use utils.JSONError() to handle errors (controller returns typed errors like BadRequest/Conflict)
}

// GetKURLMigrationStatus returns current state, phase, progress, and errors
// @Router /api/linux/kurl-migration/status [get]
func (h *Handler) GetKURLMigrationStatus(w http.ResponseWriter, r *http.Request) {
    // Call controller.GetKURLMigrationStatus(r.Context())
    // Use utils.JSON() to return KURLMigrationStatusResponse with 200
    // Use utils.JSONError() to handle errors (controller returns typed errors)
}
```

**`api/types/kurl_migration.go`** - Type definitions and errors
```go
// Error constants
var (
    ErrNoActiveKURLMigration            = errors.New("no active kURL migration")
    ErrKURLMigrationAlreadyStarted      = errors.New("kURL migration already started")
    ErrInvalidTransferMode              = errors.New("invalid transfer mode: must be 'copy' or 'move'")
    ErrKURLMigrationPhaseNotImplemented = errors.New("kURL migration phase execution not yet implemented")
)

// KURLMigrationState: NotStarted, InProgress, Completed, Failed
// KURLMigrationPhase: Discovery, Preparation, ECInstall, DataTransfer, Completed

type StartKURLMigrationRequest struct {
    TransferMode string                     `json:"transferMode,omitempty"` // "copy" or "move", defaults to "copy"
    Config       *LinuxInstallationConfig   `json:"config,omitempty"`       // Optional config overrides
}

type StartKURLMigrationResponse struct {
    MigrationID string `json:"migrationId"`
    Message     string `json:"message"`
}

type KURLMigrationStatusResponse struct {
    State       KURLMigrationState `json:"state"`
    Phase       KURLMigrationPhase `json:"phase"`
    Message     string             `json:"message"`
    Progress    int                `json:"progress"`        // 0-100
    Error       string             `json:"error,omitempty"`
    StartedAt   string             `json:"startedAt,omitempty"`   // RFC3339
    CompletedAt string             `json:"completedAt,omitempty"` // RFC3339
}
```

**`api/controllers/kurlmigration/controller.go`** - Business logic orchestration
```go
type Controller interface {
    GetInstallationConfig(ctx context.Context) (types.LinuxInstallationConfigResponse, error)
    StartKURLMigration(ctx context.Context, transferMode types.TransferMode, config types.LinuxInstallationConfig) (string, error)
    GetKURLMigrationStatus(ctx context.Context) (types.KURLMigrationStatusResponse, error)
    Run(ctx context.Context) error
}

type KURLMigrationController struct {
    manager             kurlmigrationmanager.Manager
    store               store.Store
    installationManager linuxinstallation.InstallationManager
    logger              logrus.FieldLogger
}

func NewKURLMigrationController(opts ...ControllerOption) (*KURLMigrationController, error)

// GetInstallationConfig retrieves and merges installation configuration
func (c *KURLMigrationController) GetInstallationConfig(ctx context.Context) (types.LinuxInstallationConfigResponse, error) {
    // Call manager.GetKurlConfig() to extract kURL config with non-overlapping CIDRs
    // Call manager.GetECDefaults() to get EC defaults
    // Get user config from store (empty if not set yet)
    // Merge configs (user > kURL > defaults)
    // Return response with values/defaults/resolved
}

// StartKURLMigration initializes and starts the kURL migration process
func (c *KURLMigrationController) StartKURLMigration(ctx context.Context, transferMode types.TransferMode, config types.LinuxInstallationConfig) (string, error) {
    // Validate transfer mode using manager.ValidateTransferMode() (return types.NewBadRequestError(err) if invalid)
    // Check if kURL migration already exists (return types.NewConflictError(ErrKURLMigrationAlreadyStarted))
    // Generate UUID for kURL migration
    // Get defaults and merge with user config (resolved = user > kURL > defaults)
    // Store user-provided config for future reference
    // Initialize kURL migration in store with resolved config
    // Set initial state to NotStarted
    // Launch background goroutine with detached context
    // Return kURL migration ID immediately
}

// GetKURLMigrationStatus retrieves the current kURL migration status
func (c *KURLMigrationController) GetKURLMigrationStatus(ctx context.Context) (types.KURLMigrationStatusResponse, error) {
    // Get status from store
    // Return types.NewNotFoundError(err) if ErrNoActiveKURLMigration
    // Return status
}

// Run is the internal orchestration loop (SKELETON ONLY in this PR)
func (c *KURLMigrationController) Run(ctx context.Context) error {
    // Small delay to ensure HTTP response completes
    // Defer handles all error cases by updating kURL migration state
    // Get current state from store
    // If InProgress, resume from current phase
    // Execute phases: Discovery, Preparation, ECInstall, DataTransfer
    // Set state to Completed
    // TODO (PR sc-130983): Full phase implementations
}
```

**`api/internal/managers/kurlmigration/manager.go`** - Core operations interface
```go
type Manager interface {
    GetKurlConfig(ctx context.Context) (types.LinuxInstallationConfig, error)
    GetECDefaults(ctx context.Context) (types.LinuxInstallationConfig, error)
    MergeConfigs(user, kurl, defaults types.LinuxInstallationConfig) types.LinuxInstallationConfig
    ValidateTransferMode(mode types.TransferMode) error
    ExecutePhase(ctx context.Context, phase types.KURLMigrationPhase) error
}

type kurlMigrationManager struct {
    store               kurlmigrationstore.Store
    installationManager linuxinstallation.InstallationManager
    logger              logrus.FieldLogger
}

func NewManager(opts ...ManagerOption) Manager

// GetECDefaults delegates to installationManager.GetDefaults() to reuse existing defaults logic
func (m *kurlMigrationManager) GetECDefaults(ctx context.Context) (types.LinuxInstallationConfig, error) {
    // Call installationManager.GetDefaults(runtimeConfig)
    // Returns: AdminConsolePort: 30000, DataDirectory: /var/lib/embedded-cluster, GlobalCIDR, proxy defaults, etc.
}

// MergeConfigs merges configs with precedence: user > kURL > defaults
func (m *kurlMigrationManager) MergeConfigs(user, kurl, defaults types.LinuxInstallationConfig) types.LinuxInstallationConfig {
    // Start with defaults
    // Override with kURL values (includes non-overlapping CIDRs)
    // Override with user values (highest precedence)
    // Return merged config
}

// ValidateTransferMode checks mode is "copy" or "move"
func (m *kurlMigrationManager) ValidateTransferMode(mode types.TransferMode) error

// ExecutePhase executes a kURL migration phase (SKELETON ONLY in this PR)
func (m *kurlMigrationManager) ExecutePhase(ctx context.Context, phase types.KURLMigrationPhase) error {
    // Returns ErrKURLMigrationPhaseNotImplemented for all phases in this PR
    // TODO (PR sc-130983): Implement phase execution logic
}

// NOTE: Config validation reuses installationManager.ValidateConfig() instead of duplicating validation logic
// Validates: globalCIDR, podCIDR, serviceCIDR, networkInterface, adminConsolePort, localArtifactMirrorPort, dataDirectory
```

**`api/internal/store/kurlmigration/store.go`** - In-memory state storage
```go
// Store provides methods for storing and retrieving kURL migration state
type Store interface {
    InitializeMigration(migrationID string, transferMode string, config types.LinuxInstallationConfig) error
    GetMigrationID() (string, error)
    GetStatus() (types.KURLMigrationStatusResponse, error)
    GetUserConfig() (types.LinuxInstallationConfig, error)
    SetUserConfig(config types.LinuxInstallationConfig) error
    SetState(state types.KURLMigrationState) error
    SetPhase(phase types.KURLMigrationPhase) error
    SetError(errorMsg string) error
}

type memoryStore struct {
    mu             sync.RWMutex
    migrationID    string
    state          types.KURLMigrationState
    phase          types.KURLMigrationPhase
    transferMode   string
    config         types.LinuxInstallationConfig
    userConfig     types.LinuxInstallationConfig
    errorMsg       string
    startedAt      time.Time
    completedAt    *time.Time
}

func NewMemoryStore() Store

// InitializeMigration creates new kURL migration, returns ErrKURLMigrationAlreadyStarted if exists
func (s *memoryStore) InitializeMigration(migrationID string, transferMode string, config types.LinuxInstallationConfig) error

// GetMigrationID returns current kURL migration ID, or ErrNoActiveKURLMigration if none exists
func (s *memoryStore) GetMigrationID() (string, error)

// GetStatus returns current kURL migration status with all fields
func (s *memoryStore) GetStatus() (types.KURLMigrationStatusResponse, error)

// GetUserConfig returns user-provided config (empty if not set)
func (s *memoryStore) GetUserConfig() (types.LinuxInstallationConfig, error)

// SetUserConfig stores user-provided config for reference
func (s *memoryStore) SetUserConfig(config types.LinuxInstallationConfig) error

// SetState updates kURL migration state, sets CompletedAt for Completed/Failed states
func (s *memoryStore) SetState(state types.KURLMigrationState) error

// SetPhase updates current kURL migration phase
func (s *memoryStore) SetPhase(phase types.KURLMigrationPhase) error

// SetError sets error message (state is updated separately via SetState)
func (s *memoryStore) SetError(errorMsg string) error
```

**To Be Implemented:**

**CLI-API Integration Pattern:**
The API leverages existing kURL detection utilities from the `pkg-new/kurl` package (implemented in story sc-130962). The CLI handles password export via `exportKurlPasswordHash()`, while the API focuses on configuration extraction and kURL migration orchestration. This separation ensures the API doesn't duplicate CLI detection logic.

**`api/internal/managers/kurlmigration/kurl_config.go`** - Extract kURL configuration
```go
import (
    "github.com/replicatedhq/embedded-cluster/pkg-new/kurl"
)

// GetKurlConfig extracts configuration from kURL cluster and returns EC-ready config with non-overlapping CIDRs
func (m *kurlMigrationManager) GetKurlConfig(ctx context.Context) (types.LinuxInstallationConfig, error) {
    // Use existing pkg-new/kurl.GetConfig() to get base kURL configuration
    // Extract kURL's pod/service CIDRs from kube-controller-manager
    // Extract admin console port, proxy settings from kotsadm resources
    // Calculate NEW non-overlapping CIDRs for EC (using calculateNonOverlappingCIDRs)
    // Return LinuxInstallationConfig with EC-ready values
    // NOTE: Password hash is handled by CLI (exportKurlPasswordHash), not API
    // NOTE: Install directory comes from kurl.Config.InstallDir (already retrieved from ConfigMap)
}

// extractKurlNetworkConfig extracts kURL's existing pod and service CIDRs from kube-controller-manager pod
func extractKurlNetworkConfig(ctx context.Context, kurlClient client.Client) (podCIDR, serviceCIDR string, err error)

// discoverKotsadmNamespace finds the namespace containing kotsadm Service
// Checks "default" first, then searches all namespaces for Service with label "app=kotsadm"
// Returns namespace name or error if not found
func discoverKotsadmNamespace(ctx context.Context, kurlClient client.Client) (string, error)

// extractAdminConsolePort discovers and gets NodePort from kotsadm Service
// Uses discoverKotsadmNamespace to handle vendors who deploy KOTS outside default namespace
func extractAdminConsolePort(ctx context.Context, kurlClient client.Client) (int, error)

// extractProxySettings discovers kotsadm Deployment and gets HTTP_PROXY/HTTPS_PROXY env vars
// Uses same namespace discovery logic as extractAdminConsolePort
func extractProxySettings(ctx context.Context, kurlClient client.Client) (*ProxyConfig, error)

// extractNetworkInterface reads network_interface from kube-system/kurl ConfigMap
func extractNetworkInterface(ctx context.Context, kurlClient client.Client) (string, error)
```

**`api/internal/managers/kurlmigration/network.go`** - CIDR calculation logic
```go
// calculateNonOverlappingCIDRs finds new CIDRs that don't overlap with kURL's existing ranges
func calculateNonOverlappingCIDRs(kurlPodCIDR, kurlServiceCIDR, globalCIDR string) (newPodCIDR, newServiceCIDR string, err error) {
    // Build exclusion list from kURL CIDRs
    // Find non-overlapping pod CIDR within globalCIDR
    // Find non-overlapping service CIDR within globalCIDR
    // Return new CIDRs that don't conflict with kURL
}

// findNextAvailableCIDR searches for available CIDR by incrementing through address space
func findNextAvailableCIDR(startCIDR string, excludeRanges []string, globalCIDR string) (string, error)

// overlaps checks if two CIDRs overlap using net.IPNet
func overlaps(cidr1, cidr2 string) (bool, error)

// isWithinGlobal checks if CIDR is fully contained within global CIDR
func isWithinGlobal(cidr, globalCIDR string) (bool, error)

// incrementCIDR increments CIDR block by its size (e.g., 10.32.0.0/20 -> 10.48.0.0/20)
func incrementCIDR(cidr string) (string, error)
```


### Handlers/Controllers
- kURL migration handlers are Linux-only (not available for Kubernetes target)
- Registered under authenticated routes with logging middleware at `/api/linux/kurl-migration/`
- Swagger/OpenAPI definitions included via handler annotations

### Toggle Strategy
- Feature flag: None required (Linux-only feature)
- Entitlement: None required (available to all Linux installations)
- Controlled by InstallTarget == "linux" check

### External Contracts
**APIs Consumed:**
- Kubernetes API (via controller-runtime client) for kURL config extraction
- Existing LinuxInstallationManager for EC defaults

**Events Emitted:**
- None in this PR (metrics reporting in future PR)

## Testing

### Unit Tests
**Controller Tests** (`api/controllers/kurlmigration/controller_test.go`):
- GetInstallationConfig with various config combinations
- StartKURLMigration with different transfer modes (copy/move)
- kURL migration already in progress returns 409 conflict (ErrKURLMigrationAlreadyStarted)
- Invalid transfer mode returns 400 bad request
- GetKURLMigrationStatus with active/inactive kURL migrations
- Background goroutine execution and state transitions

**Manager Tests** (`api/internal/managers/kurlmigration/manager_test.go`):
- Config merging precedence (user > kURL > defaults)
- Transfer mode validation (copy/move only)
- GetECDefaults delegates to InstallationManager.GetDefaults()
- MergeConfigs properly overrides with correct precedence
- ExecutePhase returns ErrKURLMigrationPhaseNotImplemented (skeleton in this PR)

**Network Tests** (`api/internal/managers/kurlmigration/network_test.go` - CRITICAL):
- `TestCalculateNonOverlappingCIDRs_ExcludesKurlRanges()` - Verify EC ranges don't overlap kURL
- `TestCalculateNonOverlappingCIDRs_MultipleExclusions()` - Test with multiple excluded ranges
- `TestCalculateNonOverlappingCIDRs_WithinGlobalCIDR()` - Verify calculated ranges respect global CIDR
- `TestCalculateNonOverlappingCIDRs_NoAvailableRange()` - Handle exhaustion scenarios

**kURL Config Tests** (`api/internal/managers/kurlmigration/kurl_config_test.go`):
- discoverKotsadmNamespace finds Service in default namespace first
- discoverKotsadmNamespace falls back to searching all namespaces
- extractAdminConsolePort reads NodePort from discovered Service
- extractProxySettings reads env vars from kotsadm Deployment

**Store Tests** (`api/internal/store/kurlmigration/store_test.go`):
- NewMemoryStore initialization
- Thread-safe concurrent access (multiple goroutines reading/writing)
- State transitions (NotStarted → InProgress → Completed/Failed)
- InitializeMigration returns ErrKURLMigrationAlreadyStarted when exists
- GetMigrationID returns ErrNoActiveKURLMigration when no kURL migration exists
- GetUserConfig/SetUserConfig for storing user-provided configuration

### Integration Tests
- End-to-end API flow simulation
- Background goroutine execution
- Error propagation through layers

### Dryrun Tests
**Extend existing test:** `tests/dryrun/upgrade_kurl_migration_test.go::TestUpgradeKURLMigration`

The existing test validates CLI kURL migration detection. Extend it to test the kURL migration API foundation:

```go
func TestUpgradeKURLMigration(t *testing.T) {
    // Existing setup: ENABLE_V3=1, mock kURL kubeconfig, dryrun.KubeUtils
    // Existing setup: Create kurl-config ConfigMap in kube-system namespace
    // Existing setup: Create kotsadm Service and kotsadm-password Secret in default namespace

    // Test kURL Migration API endpoints
    t.Run("migration API skeleton", func(t *testing.T) {
        // Start the upgrade command in non-headless mode so API stays up
        // Build API client and authenticate with password

        // POST /api/linux/kurl-migration/start with transferMode="copy"
        // Verify response: migrationID returned with 200
        // Verify response message: "kURL migration started successfully"

        // GET /api/linux/kurl-migration/status (with polling)
        // Verify kURL migration eventually reaches Failed state
        // Verify error contains "kURL migration phase execution not yet implemented"
        // This validates ErrKURLMigrationPhaseNotImplemented is properly returned
    })
}
```

**Test Pattern:**
- Uses `dryrun.KubeUtils{}` for mock Kubernetes clients
- Creates kURL kubeconfig at `kubeutils.KURLKubeconfigPath`
- Creates `kurl-config` ConfigMap: `Data["kurl_install_directory"] = "/var/lib/kurl"`
- Creates `kotsadm` Service and `kotsadm-password` Secret for authentication
- Uses `embedReleaseData()` helper for release artifacts
- Runs upgrade command with `--yes` flag in goroutine
- Waits for API to be ready, then tests endpoints

**CIDR Exclusion Test** (critical validation):
```go
func Test_CalculateNonOverlappingCIDRs(t *testing.T) {
    tests := []struct {
        name                string
        kurlPodCIDR         string
        kurlServiceCIDR     string
        globalCIDR          string
        expectedECPodCIDR   string
        expectedECServiceCIDR string
        shouldNotOverlap    bool
    }{
        {
            name:                "excludes kURL ranges",
            kurlPodCIDR:         "10.32.0.0/20",
            kurlServiceCIDR:     "10.96.0.0/12",
            globalCIDR:          "10.0.0.0/8",
            expectedECPodCIDR:   "10.48.0.0/20",       // Different from kURL
            expectedECServiceCIDR: "10.112.0.0/12",   // Different from kURL
            shouldNotOverlap:    true,
        },
    }
    // Verify EC CIDRs don't overlap with kURL CIDRs
}
```

## Backward compatibility

- **API Versioning**: New endpoints, no existing API changes
- **Data Format**: Reuses existing LinuxInstallationConfig type
- **Migration Windows**: N/A for this PR

## Migrations

No special deployment handling required. The API endpoints will be available immediately upon deployment.

## Trade-offs

**Chosen Approach: Async Background Processing**
- Optimizing for: UI responsiveness, handling long-running kURL migration operations
- Trade-off: Complexity of status polling vs simplicity of synchronous calls
- Rationale: kURL migration can take 30+ minutes, sync calls would timeout

**Chosen Approach: In-Memory Store (this PR)**
- Optimizing for: Simplicity, fast iteration
- Trade-off: No persistence across restarts (added in PR sc-130972)
- Rationale: Allows testing kURL migration API flow before adding persistence complexity

**Chosen Approach: Three-Endpoint Design**
- Optimizing for: Clear separation of concerns, RESTful design
- Trade-off: More endpoints vs single GraphQL-style endpoint
- Rationale: Follows existing API patterns at `/api/linux/kurl-migration/`, easier to test/document

## Alternative solutions considered

1. **Single /migrate Endpoint with WebSocket**
   - Rejected: Adds WebSocket complexity, inconsistent with existing patterns

2. **Synchronous kURL Migration Execution**
   - Rejected: Would timeout on long kURL migrations, poor UX

3. **Direct UI to Controller Communication**
   - Rejected: Breaks architectural layers, harder to test

4. **GraphQL API**
   - Rejected: Inconsistent with REST-based architecture

5. **Separate kURL Migration Service**
   - Rejected: Adds deployment complexity, harder to maintain

## Research

See detailed research document: [kurl-migration-api-foundation_research.md](./kurl-migration-api-foundation_research.md)

### Prior Art in Codebase
- Linux installation API pattern: `/api/controllers/linux/install/`
- Async operation pattern: Airgap processing in `/api/internal/managers/airgap/`
- Config merging: Installation config resolution in managers
- Status polling: Installation and upgrade status endpoints

### External References
- [Kubernetes Client-go ConfigMap Access](https://github.com/kubernetes/client-go/blob/master/examples/create-update-delete-configmap/main.go)
- [CIDR Overlap Detection Algorithm](https://github.com/containernetworking/plugins/blob/main/pkg/ip/cidr.go)
- [Go Async Pattern Best Practices](https://github.com/golang/go/wiki/CommonMistakes#using-goroutines-on-loop-iterator-variables)

## Checkpoints (PR plan)

**This PR (sc-130971): kURL Migration API Foundation**
- Complete handler implementation with Swagger docs at `/api/linux/kurl-migration/`
- Controller with async execution (background goroutine with Run method)
- Manager with config merging and validation
- In-memory store implementation (memoryStore)
- Skeleton phase execution (returns ErrKURLMigrationPhaseNotImplemented)
- Comprehensive unit tests and dryrun tests
- Sets foundation for subsequent kURL migration PRs

**Future PRs (not in this PR):**
- PR sc-130972: Add persistent file-based store at `/var/lib/embedded-cluster/migration-state.json`
- PR sc-130983: Implement kURL migration phase orchestration (Discovery, Preparation, ECInstall, DataTransfer)
- Future PRs: Add kURL config extraction, CIDR calculation, metrics reporting