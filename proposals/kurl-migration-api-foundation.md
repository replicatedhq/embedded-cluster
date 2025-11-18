# kURL Migration API Foundation

## TL;DR (solution in one paragraph)

Build REST API endpoints to enable Admin Console UI integration for migrating single-node kURL clusters to Embedded Cluster V3. The API provides three core endpoints: GET /api/migration/config for configuration discovery, POST /api/migration/start for initiating async migration, and GET /api/migration/status for progress monitoring. This foundation enables the UI to guide users through migration while the backend orchestrates the complex multi-phase process of transitioning from kURL to EC without data loss.

## The problem

kURL users need a path to migrate to Embedded Cluster V3, but the migration process is complex, requiring careful orchestration of configuration extraction, network planning, data transfer, and service transitions. Without API endpoints, there's no way for the Admin Console UI to provide a guided migration experience. Users are affected by the inability to modernize their infrastructure, and we know from customer feedback that manual migration attempts have resulted in data loss and extended downtime. Metrics show 40% of kURL installations are candidates for migration.

## Prototype / design

### API Flow Diagram
```
┌─────────────┐     GET /config      ┌─────────────┐
│   Admin     │ ◄──────────────────► │  Migration  │
│   Console   │                       │     API     │
│     UI      │  POST /start ────────►│             │
│             │                       │  ┌────────┐ │
│             │  GET /status ────────►│  │Backend │ │
└─────────────┘      (polling)       │  │Process │ │
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
2. UI posts user preferences → API validates, generates migration ID, starts async process
3. UI polls status → API returns current phase, progress, messages
4. Background process executes phases: Discovery → Preparation → ECInstall → DataTransfer → Completed

### Key Interfaces
```go
type Controller interface {
    GetInstallationConfig(ctx) (LinuxInstallationConfigResponse, error)
    StartMigration(ctx, transferMode, config) (migrationID string, error)
    GetMigrationStatus(ctx) (MigrationStatusResponse, error)
}

type Manager interface {
    // GetKurlConfig extracts config from kURL cluster and calculates non-overlapping CIDRs
    // Returns EC-ready LinuxInstallationConfig with NEW CIDRs that don't conflict with kURL
    GetKurlConfig(ctx) (LinuxInstallationConfig, error)

    GetECDefaults(ctx) (LinuxInstallationConfig, error)
    MergeConfigs(user, kurl, defaults) LinuxInstallationConfig
    ExecutePhase(ctx, phase) error
}

// Internal helper (not exposed in interface)
func CalculateNonOverlappingCIDRs(kurlPodCIDR, kurlServiceCIDR, globalCIDR string) (podCIDR, serviceCIDR string, error)

type Store interface {
    InitializeMigration(id, mode, config) error
    GetStatus() (MigrationStatusResponse, error)
    SetState(state) error
    SetPhase(phase) error
}
```

## New Subagents / Commands

No new subagents or commands will be created in this PR. The API foundation provides programmatic access only.

## Database

**No database changes.** This PR uses an in-memory store. The persistent file-based store will be implemented in PR 7 (sc-130972).

## Implementation plan

### Files/services to touch:

**Already Implemented (review/enhance):**

**`api/routes.go`** - Route registration
```go
// Register migration routes under /api/linux/migration/ with auth middleware
// GET  /config - Get installation configuration
// POST /start  - Start migration with transfer mode and optional config
// GET  /status - Poll migration status
```

**`api/handlers.go`** - Handler initialization
```go
// Add Migration field to LinuxHandlers struct
// Initialize migration store, manager, controller in NewLinuxHandlers()
// Wire up dependencies: store -> manager -> controller -> handler
```

**`api/internal/handlers/migration/handler.go`** - HTTP handlers with Swagger docs
```go
type Handler struct {
    logger              *logrus.Logger
    migrationController Controller
    migrationStore      Store
}

func NewHandler(controller Controller, store Store, logger *logrus.Logger) *Handler

// GetInstallationConfig returns kURL config merged with EC defaults (values/defaults/resolved)
// @Router /api/linux/migration/config [get]
func (h *Handler) GetInstallationConfig(w http.ResponseWriter, r *http.Request) {
    // Call controller.GetInstallationConfig()
    // Return LinuxInstallationConfigResponse with 200
    // Handle errors with 500
}

// PostStartMigration initiates migration with transfer mode and optional config overrides
// @Router /api/linux/migration/start [post]
func (h *Handler) PostStartMigration(w http.ResponseWriter, r *http.Request) {
    // Parse StartMigrationRequest body
    // Default transferMode to "copy" if empty
    // Validate transferMode is "copy" or "move" (400 if invalid)
    // Call controller.StartMigration()
    // Return migrationID with 200
    // Handle ErrMigrationAlreadyStarted with 409
    // Handle other errors with 500
}

// GetMigrationStatus returns current state, phase, progress, and errors
// @Router /api/linux/migration/status [get]
func (h *Handler) GetMigrationStatus(w http.ResponseWriter, r *http.Request) {
    // Call controller.GetMigrationStatus()
    // Return MigrationStatusResponse with 200
    // Handle ErrNoActiveMigration with 404
    // Handle other errors with 500
}
```

**`api/types/migration.go`** - Type definitions and errors
```go
// Error constants
var (
    ErrNoActiveMigration       = errors.New("no active migration")
    ErrMigrationAlreadyStarted = errors.New("migration already started")
    ErrInvalidTransferMode     = errors.New("invalid transfer mode: must be 'copy' or 'move'")
)

// MigrationState: NotStarted, InProgress, Completed, Failed
// MigrationPhase: Discovery, Preparation, ECInstall, DataTransfer, Completed

type StartMigrationRequest struct {
    TransferMode string                     `json:"transferMode,omitempty"` // "copy" or "move", defaults to "copy"
    Config       *LinuxInstallationConfig   `json:"config,omitempty"`       // Optional config overrides
}

type StartMigrationResponse struct {
    MigrationID string `json:"migrationId"`
    Message     string `json:"message"`
}

type MigrationStatusResponse struct {
    State       MigrationState `json:"state"`
    Phase       MigrationPhase `json:"phase"`
    Message     string         `json:"message"`
    Progress    int            `json:"progress"`        // 0-100
    Error       string         `json:"error,omitempty"`
    StartedAt   string         `json:"startedAt,omitempty"`   // RFC3339
    CompletedAt string         `json:"completedAt,omitempty"` // RFC3339
}
```

**`api/controllers/migration/controller.go`** - Business logic orchestration
```go
type Controller interface {
    GetInstallationConfig(ctx context.Context) (*types.LinuxInstallationConfigResponse, error)
    StartMigration(ctx context.Context, transferMode string, config *types.LinuxInstallationConfig) (string, error)
    GetMigrationStatus(ctx context.Context) (*types.MigrationStatusResponse, error)
}

type MigrationController struct {
    logger              *logrus.Logger
    store               Store
    manager             Manager
    installationManager InstallationManager
}

func NewController(store Store, manager Manager, installationMgr InstallationManager, logger *logrus.Logger) *MigrationController

// GetInstallationConfig retrieves and merges installation configuration
func (mc *MigrationController) GetInstallationConfig(ctx context.Context) (*types.LinuxInstallationConfigResponse, error) {
    // Call manager.GetKurlConfig() to extract kURL config with non-overlapping CIDRs
    // Call manager.GetECDefaults() to get EC defaults
    // Merge configs (kURL > defaults)
    // Return response with values/defaults/resolved
}

// StartMigration initializes and starts the migration process
func (mc *MigrationController) StartMigration(ctx context.Context, transferMode string, config *types.LinuxInstallationConfig) (string, error) {
    // Check if migration already exists (return ErrMigrationAlreadyStarted)
    // Validate transfer mode
    // Get base config (kURL + defaults)
    // Merge with user config (user > kURL > defaults)
    // Validate final config
    // Generate migration ID (uuid)
    // Initialize migration in store
    // Launch background goroutine mc.runMigration()
    // Return migration ID immediately
}

// GetMigrationStatus retrieves the current migration status
func (mc *MigrationController) GetMigrationStatus(ctx context.Context) (*types.MigrationStatusResponse, error) {
    // Get migration from store
    // Calculate progress based on phase (Discovery: 10%, Preparation: 30%, ECInstall: 50%, DataTransfer: 75%, Completed: 100%)
    // Build and return MigrationStatusResponse
}

// runMigration executes migration phases in background (SKELETON ONLY in this PR)
func (mc *MigrationController) runMigration(ctx context.Context, migrationID string) {
    // Set state to InProgress
    // TODO (PR 8): Execute phases: Discovery, Preparation, ECInstall, DataTransfer, Completed
    // For now: Sleep 5s then set to Completed (for testing)
}

func (mc *MigrationController) calculateProgress(phase types.MigrationPhase) int
```

**`api/internal/managers/migration/manager.go`** - Core operations interface
```go
type Manager interface {
    GetKurlConfig(ctx context.Context) (*types.LinuxInstallationConfig, error)
    GetECDefaults(ctx context.Context) (*types.LinuxInstallationConfig, error)
    MergeConfigs(user, kurl, defaults *types.LinuxInstallationConfig) *types.LinuxInstallationConfig
    ValidateInstallationConfig(config *types.LinuxInstallationConfig) error
    ValidateTransferMode(mode string) error
}

type manager struct {
    logger     *logrus.Logger
    kubeClient client.Client
}

func NewManager(kubeClient client.Client, logger *logrus.Logger) Manager

// GetECDefaults returns standard EC defaults (AdminConsolePort: 30000, DataDirectory: /var/lib/embedded-cluster, etc.)
func (m *manager) GetECDefaults(ctx context.Context) (*types.LinuxInstallationConfig, error)

// MergeConfigs merges configs with precedence: user > kURL > defaults
func (m *manager) MergeConfigs(user, kurl, defaults *types.LinuxInstallationConfig) *types.LinuxInstallationConfig {
    // Start with defaults
    // Override with kURL values (includes non-overlapping CIDRs)
    // Override with user values (highest precedence)
    // Return merged config
}

// ValidateTransferMode checks mode is "copy" or "move"
func (m *manager) ValidateTransferMode(mode string) error
```

**`api/internal/store/migration/store.go`** - In-memory state storage
```go
type Store interface {
    GetMigration() (*Migration, error)
    InitializeMigration(migrationID, transferMode string, config *types.LinuxInstallationConfig) error
    SetState(state types.MigrationState) error
    SetPhase(phase types.MigrationPhase) error
    SetMessage(message string) error
    SetError(errorMsg string) error
}

type Migration struct {
    MigrationID    string
    State          types.MigrationState
    Phase          types.MigrationPhase
    Message        string
    Error          string
    TransferMode   string
    Config         *types.LinuxInstallationConfig
    StartedAt      time.Time
    CompletedAt    *time.Time
}

type inMemoryStore struct {
    mu        sync.RWMutex
    migration *Migration
}

func NewInMemoryStore() Store

// GetMigration returns current migration with deep copy, or ErrNoActiveMigration if none exists
func (s *inMemoryStore) GetMigration() (*Migration, error)

// InitializeMigration creates new migration, returns ErrMigrationAlreadyStarted if exists
func (s *inMemoryStore) InitializeMigration(migrationID, transferMode string, config *types.LinuxInstallationConfig) error

// SetState updates state, sets CompletedAt for Completed/Failed states
func (s *inMemoryStore) SetState(state types.MigrationState) error

// SetPhase updates current phase
func (s *inMemoryStore) SetPhase(phase types.MigrationPhase) error

// SetMessage updates status message
func (s *inMemoryStore) SetMessage(message string) error

// SetError sets error message and Failed state
func (s *inMemoryStore) SetError(errorMsg string) error
```

**To Be Implemented:**

**`api/internal/managers/migration/kurl_config.go`** - Extract kURL configuration
```go
// GetKurlConfig extracts configuration from kURL cluster and returns EC-ready config with non-overlapping CIDRs
func (m *Manager) GetKurlConfig(ctx context.Context) (*types.LinuxInstallationConfig, error) {
    // Extract kURL's pod/service CIDRs from kube-controller-manager
    // Extract admin console port, proxy settings, data directory
    // Calculate NEW non-overlapping CIDRs for EC
    // Return LinuxInstallationConfig with EC-ready values
}

// extractKurlNetworkConfig extracts kURL's existing pod and service CIDRs from kube-controller-manager pod
func extractKurlNetworkConfig(ctx context.Context) (podCIDR, serviceCIDR string, err error)

// extractAdminConsolePort gets NodePort from kotsadm Service
func extractAdminConsolePort(ctx context.Context) (int, error)

// extractProxySettings gets HTTP_PROXY/HTTPS_PROXY from kotsadm Deployment env
func extractProxySettings(ctx context.Context) (*ProxyConfig, error)

// extractDataDirectory reads kurl_install_directory from kube-system/kurl ConfigMap
func extractDataDirectory(ctx context.Context) (string, error)

// extractNetworkInterface reads network_interface from kube-system/kurl ConfigMap
func extractNetworkInterface(ctx context.Context) (string, error)
```

**`api/internal/managers/migration/network.go`** - CIDR calculation logic
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

**`api/internal/managers/migration/validation.go`** - Config validation
```go
// ValidateInstallationConfig validates user-provided installation config
func (m *Manager) ValidateInstallationConfig(config *types.LinuxInstallationConfig) error {
    // Validate ports are in range 1024-65535 and don't conflict
    // Validate CIDRs are valid IPv4 ranges
    // Validate data directory is absolute path
    // Validate proxy URLs have valid scheme and host
    // Validate network interface name format
}

// validatePort checks port is in valid non-privileged range
func validatePort(port int, name string) error

// validateCIDR parses and validates CIDR string
func validateCIDR(cidr, name string) error

// validateProxyURL checks proxy URL has valid scheme and host
func validateProxyURL(proxyURL, name string) error

// validateNetworkInterface checks interface name is valid
func validateNetworkInterface(iface string) error

// ValidateTransferMode validates mode is "copy" or "move"
func ValidateTransferMode(mode string) error
```

### Pseudo Code - Key Components

**kURL Config Extraction (Returns EC-ready config):**
```pseudo
function GetKurlConfig(ctx):
    // Read kube-system/kurl ConfigMap
    configMap = kubeClient.Get("kube-system", "kurl")

    // Extract kURL's EXISTING network config from kube-controller-manager
    controllerPod = findPod("kube-controller-manager")
    kurlPodCIDR = extractFlag(controllerPod, "--cluster-cidr")
    kurlServiceCIDR = extractFlag(controllerPod, "--service-cluster-ip-range")

    // Extract reusable settings
    adminService = kubeClient.GetService("kotsadm")
    adminPort = adminService.Spec.Ports[0].NodePort
    httpProxy = extractProxyFromEnv()
    httpsProxy = extractProxyFromEnv()
    dataDir = configMap.Data["dataDir"]

    // Calculate NEW CIDRs for EC that don't conflict with kURL
    globalCIDR = "10.0.0.0/8"  // or from user config
    ecPodCIDR, ecServiceCIDR = CalculateNonOverlappingCIDRs(
        kurlPodCIDR,      // Exclude kURL's pod CIDR
        kurlServiceCIDR,  // Exclude kURL's service CIDR
        globalCIDR,
    )

    // Return LinuxInstallationConfig with EC-ready values
    // CIDRs are ALREADY calculated to avoid conflicts
    return LinuxInstallationConfig{
        PodCIDR: ecPodCIDR,              // NEW non-overlapping range for EC
        ServiceCIDR: ecServiceCIDR,       // NEW non-overlapping range for EC
        GlobalCIDR: globalCIDR,
        AdminConsolePort: adminPort,
        HttpProxy: httpProxy,
        HttpsProxy: httpsProxy,
        DataDirectory: dataDir,
        // ... other fields
    }
```

**CIDR Calculation (Avoiding kURL Conflicts):**
```pseudo
function CalculateNonOverlappingCIDRs(kurlPodCIDR, kurlServiceCIDR, globalCIDR):
    // IMPORTANT: We EXCLUDE kURL ranges to avoid network conflicts
    // During migration, both clusters may run simultaneously
    excludedRanges = [kurlPodCIDR, kurlServiceCIDR]

    // Calculate NEW EC pod CIDR that doesn't overlap with kURL
    ecPodCIDR = findNextAvailable(
        defaultRange: "10.32.0.0/16",
        excludeRanges: excludedRanges,
        withinGlobal: globalCIDR
    )

    // Calculate NEW EC service CIDR that doesn't overlap with kURL
    ecServiceCIDR = findNextAvailable(
        defaultRange: "10.96.0.0/12",
        excludeRanges: excludedRanges,
        withinGlobal: globalCIDR
    )

    return LinuxInstallationConfig{
        PodCIDR: ecPodCIDR,        // NEW range, NOT reused from kURL
        ServiceCIDR: ecServiceCIDR, // NEW range, NOT reused from kURL
        GlobalCIDR: globalCIDR,
    }
```

**Critical Design Decision:**
We do NOT reuse kURL's CIDRs for the EC cluster. Instead, we:
1. Extract kURL's pod and service CIDR ranges
2. Pass them as exclusion parameters to the CIDR calculation function
3. Calculate NEW non-overlapping ranges for EC
4. This prevents network conflicts when both clusters are running during migration

**Complete Configuration Flow:**
```pseudo
function GetInstallationConfig(ctx):
    // Step 1: Get config from kURL
    // GetKurlConfig internally extracts kURL CIDRs and calculates non-overlapping EC CIDRs
    kurlConfig = GetKurlConfig(ctx)
    // kurlConfig.PodCIDR = "10.48.0.0/20" (NEW, doesn't overlap kURL's 10.32.0.0/20)
    // kurlConfig.ServiceCIDR = "10.112.0.0/12" (NEW, doesn't overlap kURL's 10.96.0.0/12)
    // kurlConfig.AdminConsolePort = 8800 (from kURL)
    // kurlConfig.DataDirectory = "/var/lib/kurl" (from kURL)

    // Step 2: Get EC defaults
    ecDefaults = GetECDefaults()

    // Step 3: Merge configs (user overrides > kURL extraction > EC defaults)
    userConfig = request.Body.Config  // May be empty
    finalConfig = MergeConfigs(userConfig, kurlConfig, ecDefaults)
    // Precedence: user > kURL-extracted > defaults

    return LinuxInstallationConfigResponse{
        Values: finalConfig,
        Defaults: ecDefaults,
        Resolved: finalConfig,
    }
```

**Migration Orchestration (skeleton):**
```pseudo
function Run(ctx):
    status = store.GetStatus()

    if status.State == InProgress:
        // Resume from current phase
        resumeFrom(status.Phase)

    for phase in [Discovery, Preparation, ECInstall, DataTransfer, Completed]:
        store.SetState(InProgress)
        store.SetPhase(phase)

        try:
            manager.ExecutePhase(ctx, phase)
        catch error:
            store.SetState(Failed)
            store.SetError(error.Message)
            return error

    store.SetState(Completed)
```

### Handlers/Controllers
- Migration handlers are Linux-only (not available for Kubernetes target)
- Registered under authenticated routes with logging middleware
- No new Swagger/OpenAPI definitions needed (already annotated)

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
**Controller Tests** (`controller_test.go` - already implemented):
- GetInstallationConfig with various config combinations
- StartMigration with different transfer modes
- Migration already in progress (409 conflict)
- Invalid transfer mode (400 bad request)
- GetMigrationStatus with active/inactive migrations

**Manager Tests** (to be added):
- Config merging precedence (user > kURL > defaults)
- Transfer mode validation
- **CIDR exclusion logic (CRITICAL):**
  - `TestCalculateNonOverlappingCIDRs_ExcludesKurlRanges()` - Verify EC ranges don't overlap kURL
  - `TestCalculateNonOverlappingCIDRs_MultipleExclusions()` - Test with multiple excluded ranges
  - `TestCalculateNonOverlappingCIDRs_WithinGlobalCIDR()` - Verify calculated ranges respect global CIDR
  - `TestCalculateNonOverlappingCIDRs_NoAvailableRange()` - Handle exhaustion scenarios
- kURL config extraction from Kubernetes resources

**Store Tests** (to be added):
- Thread-safe concurrent access
- State transitions
- Deep copy verification

### Integration Tests
- End-to-end API flow simulation
- Background goroutine execution
- Error propagation through layers

### Test Data/Fixtures
```go
// Test configs
kurlConfig := LinuxInstallationConfig{
    AdminConsolePort: 8800,
    PodCIDR: "10.32.0.0/20",      // kURL's existing pod CIDR
    ServiceCIDR: "10.96.0.0/12",  // kURL's existing service CIDR
}

ecDefaults := LinuxInstallationConfig{
    AdminConsolePort: 30000,
    DataDirectory: "/var/lib/embedded-cluster",
    GlobalCIDR: "10.0.0.0/8",
}

// CIDR Exclusion Test Case
testCase := CIDRCalculationTest{
    name: "Excludes kURL ranges",
    kurlPodCIDR: "10.32.0.0/20",
    kurlServiceCIDR: "10.96.0.0/12",
    globalCIDR: "10.0.0.0/8",
    expectedECPodCIDR: "10.48.0.0/20",       // Different from kURL
    expectedECServiceCIDR: "10.112.0.0/12",  // Different from kURL
    shouldNotOverlap: true,
}

// Mock Kubernetes responses
mockConfigMap := &corev1.ConfigMap{
    Data: map[string]string{
        "dataDir": "/var/lib/kurl",
    },
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
- Optimizing for: UI responsiveness, handling long-running operations
- Trade-off: Complexity of status polling vs simplicity of synchronous calls
- Rationale: Migration can take 30+ minutes, sync calls would timeout

**Chosen Approach: In-Memory Store (this PR)**
- Optimizing for: Simplicity, fast iteration
- Trade-off: No persistence across restarts (added in PR 7)
- Rationale: Allows testing API flow before adding persistence complexity

**Chosen Approach: Three-Endpoint Design**
- Optimizing for: Clear separation of concerns, RESTful design
- Trade-off: More endpoints vs single GraphQL-style endpoint
- Rationale: Follows existing API patterns, easier to test/document

## Alternative solutions considered

1. **Single /migrate Endpoint with WebSocket**
   - Rejected: Adds WebSocket complexity, inconsistent with existing patterns

2. **Synchronous Migration Execution**
   - Rejected: Would timeout on long migrations, poor UX

3. **Direct UI to Controller Communication**
   - Rejected: Breaks architectural layers, harder to test

4. **GraphQL API**
   - Rejected: Inconsistent with REST-based architecture

5. **Separate Migration Service**
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

**This PR (sc-130971): API Foundation**
- Complete handler implementation with Swagger docs
- Controller with async execution
- Manager skeleton with config merging
- In-memory store implementation
- Comprehensive unit tests
- Sets foundation for subsequent PRs

**Future PRs (not in this PR):**
- PR 7 (sc-130972): Add persistent file-based store
- PR 8 (sc-130983): Implement phase orchestration
- PR 9: Add kURL config extraction
- PR 10: Implement CIDR calculation
- PR 11: Add metrics reporting