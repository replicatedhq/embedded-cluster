# Proposal: Transition Application Deployment to V3 Embedded-Cluster

**Status:** Proposed
**Epic:** [Installer Experience v2 - Milestone 7](https://app.shortcut.com/replicated/epic/126565)
**Related Stories:**

| Story | Description |
|-------|-------------|
| sc-128049 | Update KOTS to not deploy the app for v3 EC installs |
| sc-128065 | Make sure the values and optionalValues fields of the HelmChart custom resource are respected |
| sc-128045 | Update app install endpoint to install Helm charts directly without KOTS CLI |
| sc-128062 | Support the releaseName field in the HelmChart custom resource |
| sc-128364 | Rely on KOTS CLI to process the app's airgap bundle and create the image pull secrets |
| sc-128060 | Add missing functionality for the image pull secret template functions |
| sc-128058 | Use Helm binary instead of the Go SDK to manage charts in V3 installs |
| sc-128450 | Support helmUpgradeFlags field from the HelmChart custom resource when deploying charts |
| sc-128057 | Sort charts by the weight field in the HelmChart custom resource |
| sc-128056 | Make sure the exclude field from the HelmChart custom resource is respected |
| sc-128055 | Make sure the namespace field in the HelmChart custom resource is respected |
| sc-128047 | Update the app installation page to show charts being installed with progress |
| sc-128046 | Update app install status endpoint to return list of charts being installed |

## TL;DR

Transition application deployment from KOTS to the V3 embedded-cluster binary to deliver Helm charts directly through our installer instead of KOTS. This enables better control, reliability, and user experience while maintaining KOTS functionality for upgrades and management, delivered through iterative milestones.

## The Problem

This proposal addresses the larger architectural goal of controlling application lifecycle management outside the cluster through the embedded-cluster binary, while ensuring KOTS continues to function end-to-end during the transition.

**Long-term vision:** Move entire application deployment and lifecycle management from in-cluster KOTS to the external embedded-cluster binary for better control, reliability, and user experience.

**Current epic goal:** Enable the V3 embedded-cluster binary to handle initial application deployment with full HelmChart resource support while allowing KOTS to seamlessly take over lifecycle management post-install.

**Current technical problem:** The V3 API installation manager delegates entirely to KOTS CLI for application deployment, preventing direct control over Helm chart installation and HelmChart custom resource configuration. If we were to add deployment logic to the V3 installer without stopping KOTS from deploying the app too, both components would attempt to deploy the app during initial install, leading to resource conflicts and unclear ownership.

## Prototype / Design

The solution transitions application deployment from KOTS to the V3 embedded-cluster binary:

### Flow Diagram

V3 Install API
   ↓
Setup Helm Client
   ↓
Install Charts (V3 Binary)
   ↓
Call KOTS CLI (Skip Deploy)

### Flow Details

1. **V3 binary takes ownership** of application deployment
2. **Setup Helm client** following infra manager patterns  
3. **Install charts directly** from releaseData.HelmChartArchives with full HelmChart custom resource support
4. **Call KOTS CLI** with SkipDeploy: true for metadata management only
5. **Fail fast** on errors without complex rollback logic

## Implementation Plan

### Iteration 1: Core Deployment Transition

#### 1.1 Story sc-128049: Update KOTS to not deploy the app for v3 EC installs

**Purpose:** Prevent KOTS from attempting to deploy applications during V3 initial installs to avoid resource conflicts.

**Implementation:**
- Add detection for V3 EC initial installs in KOTS DeployApp function
- Skip deployment and create version record when V3 is handling the deployment
- Use `IS_EMBEDDED_CLUSTER_V3` environment variable for detection
- Pass the `IS_EMBEDDED_CLUSTER_V3` environment variable to the admin console chart in V3 installer.

```go
// pkg/operator/operator.go - DeployApp method
if util.IsV3EmbeddedClusterInitialInstall(sequence) {
    // Skip deployment, create success record for admin console
    return true, nil
}
// Continue with normal KOTS deployment logic...

// pkg/util/util.go - Detection utilities
func IsV3EmbeddedCluster() bool {
    return os.Getenv("IS_EMBEDDED_CLUSTER_V3") == "true"
}

func IsV3EmbeddedClusterInitialInstall(sequence int64) bool {
	return IsV3EmbeddedCluster() && sequence == 0
}
```

In `embedded-cluster` repo (`pkg/addons/adminconsole/values.go`):

```go
// ...
copiedValues["isEmbeddedClusterV3"] = a.IsV3
// ...
```

#### 1.2 Story sc-128045: Update app install endpoint to install Helm charts directly

**Purpose:** Enable V3 binary to install Helm charts directly without using KOTS CLI.

**Implementation:**
- Add Helm client to app install manager
- Install charts from releaseData.HelmChartArchives before calling KOTS
- Call KOTS CLI with IS_EMBEDDED_CLUSTER_V3=true environment variable for metadata management only

```go
// api/internal/managers/app/install/manager.go
type appInstallManager struct {
    hcli *helm.Client  // New: Helm client for direct chart installation
    // existing fields...
}

// api/internal/managers/app/install/install.go
func (m *appInstallManager) Install(ctx context.Context) error {
    // New: Install Helm charts directly
    for _, chart := range m.releaseData.HelmChartArchives {
        err := m.installHelmChart(ctx, chart)  // install with defaults
    }
    
    // Existing: Call KOTS CLI for metadata only
    return m.kotsClient.Install(kotsInstallArgs{SkipDeploy: true})
}
```

### Iteration 2: Full HelmChart Resource Implementation

---

#### 2.1 Story sc-128065: Support values and optionalValues fields

**Purpose:** Process HelmChart CR values and optionalValues fields to configure chart installations.

**Implementation:**
- Add ExtractInstallableHelmCharts method to app release manager
- Template HelmChart CRs using existing templateHelmChartCRs function
- Find the corresponding chart archive for this HelmChart CR
- Generate Helm values from the templated CR using existing generateHelmValues function which takes care of both values and optionalValues fields
- Return installable charts with archive, processed values, and CR
- Controller orchestrates data flow between release and install managers

```go
// api/internal/managers/app/release/manager.go - New method
func (m *appReleaseManager) ExtractInstallableHelmCharts(ctx context.Context, configValues types.AppConfigValues) ([]InstallableHelmChart, error) {
   // Template Helm chart CRs with config values
	templatedCRs, err := m.templateHelmChartCRs(configValues)

    // Iterate over each templated CR and create installable chart with processed values
	for _, cr := range templatedCRs {
      // Find the corresponding chart archive for this HelmChart CR
		chartArchive, err := findChartArchive(m.releaseData.HelmChartArchives, cr)

      // Generate Helm values from the templated CR
		values, err := generateHelmValues(cr)

      // Create installable chart with archive, processed values, and CR
		installableChart := types.InstallableHelmChart{
			// ...
		}
   }
}

// api/controllers/app/install/appinstall.go - Controller orchestration
func (c *AppInstallController) Install(ctx context.Context) error {
    charts, _ := c.appReleaseManager.ExtractInstallableHelmCharts(ctx, configValues)
    return c.appInstallManager.Install(ctx, charts)
}
```

---

#### 2.2 Story sc-128062: Support releaseName field

**Purpose:** Use custom release names from HelmChart CR instead of chart name.

**Implementation:**
- Use releaseName from HelmChart CR spec when available
- Fall back to chart name as default

```go
// api/internal/managers/app/install/install.go
func (m *appInstallManager) installHelmChart(ctx context.Context, chart InstallableHelmChart) error {
   return m.hcli.Install(ctx, helm.InstallOptions{ReleaseName: chart.CR.GetReleaseName(), ...})
}
```

---

#### 2.3 Story sc-128055: Support namespace field

**Purpose:** Install charts to namespaces specified in HelmChart CR.

**Implementation:**
- Use namespace from HelmChart CR spec when available
- Create namespace if it doesn't exist
- Fall back to kotsadm namespace as default

```go
// api/internal/managers/app/install/install.go
func (m *appInstallManager) installHelmChart(ctx context.Context, chart InstallableHelmChart) error {
   // Fallback to admin console namespace if namespace is not set
   namespace := installableChart.CR.GetNamespace()
   if namespace == "" {
      namespace = constants.KotsadmNamespace
   }
   return m.hcli.Install(ctx, helm.InstallOptions{Namespace: namespace, ...})
}
```

---

#### 2.4 Story sc-128056: Support exclude field

**Purpose:** Skip installation of charts marked as excluded in HelmChart CR.

**Implementation:**
- Evaluate exclude expression using template functions during chart extraction
- Skip excluded charts from installable chart list

```go
// api/internal/managers/app/release/template.go
func (m *appReleaseManager) ExtractInstallableHelmCharts(...) ([]InstallableHelmChart, error) {
   // Iterate over each templated CR and create installable chart with processed values
	for _, cr := range templatedCRs {
      // Check if the chart should be excluded
		if !cr.Spec.Exclude.IsEmpty() {
			exclude, err := cr.Spec.Exclude.Boolean()
			if exclude {
				continue
			}
		}
   }
}
```

---

#### 2.5 Story sc-128057: Sort charts by weight field

**Purpose:** Install charts in order specified by weight field in HelmChart CR.

**Implementation:**
- Sort installable charts by weight field before returning from ExtractInstallableHelmCharts
- Use weight 0 as default for charts without weight specified

```go
// api/internal/managers/app/release/template.go
func (m *appReleaseManager) ExtractInstallableHelmCharts(...) ([]InstallableHelmChart, error) {
    // Build installable charts list...
    
    // Sort by weight before returning
    sort.Slice(installableCharts, func(i, j int) bool {
      return installableCharts[i].CR.Spec.Weight < installableCharts[j].CR.Spec.Weight
    })
}
```

---

#### 2.6 Story sc-128058: Use Helm binary instead of the Go SDK

**Purpose:** In order to facilitate the migration from KOTS with minimal risk and potential regressions, and in addition to other benefits, we should use the Helm binary instead of the Go SDK to manage charts

**Implementation:** See [helm_binary_migration.md](./helm_binary_migration.md)

#### 2.7 Story sc-128450: Support helmUpgradeFlags field

**Purpose:** Apply custom Helm upgrade flags from HelmChart CR during installation

**Implementation:**
- Pass helmUpgradeFlags directly to the helm install command arguments

```go
// api/internal/managers/app/install/install.go
func (m *appInstallManager) installHelmChart(ctx context.Context, chart InstallableHelmChart) error {
    opts := helm.InstallOptions{...}
    
    // Pass upgrade flags directly as extra args
    if len(chart.CR.Spec.HelmUpgradeFlags) > 0 {
        opts.ExtraArgs = append(opts.ExtraArgs, chart.CR.Spec.HelmUpgradeFlags...)
    }
    
    return m.hcli.Install(ctx, opts)
}

// pkg/helm/binary_client.go
func (c *BinaryHelmClient) Install(ctx context.Context, opts InstallOptions) (*release.Release, error) {
    args := []string{"install", opts.ReleaseName}
    
    // ... existing code ...
    
    // Pass extra args
    if len(opts.ExtraArgs) > 0 {
        args = append(args, opts.ExtraArgs...)
    }
    
    // Execute helm command
    stdout, stderr, err := c.executor.ExecuteCommand(ctx, nil, c.helmPath, args...)
    
    // Parse release from JSON output
    return &release, nil
}
```

### Iteration 3: Complete Registry Integration

---

#### 3.1 Story sc-128364: Rely on KOTS CLI to process the app's airgap bundle and create the image pull secrets

**Purpose:** Since we're cutting airgap bundle processing and image pull secret creation out of scope for this epic, we need to keep relying on KOTS CLI to achieve this.

**Implementation:**
- Call the `kots install` CLI command before we install the app's helm charts: https://github.com/replicatedhq/embedded-cluster/blob/445ac7500f9eef2e958596eea59d119df559471f/api/internal/managers/app/install/install.go#L47
- KOTS will process the airgap bundle and create the image pull secrets in the cluster without deploying the application.

```go
// api/internal/managers/app/install/install.go
func (m *appInstallManager) install(ctx context.Context, installableCharts []types.InstallableHelmChart, kotsConfigValues kotsv1beta1.ConfigValues) error {
   // Move before the chart installation
   // ...
	if err := kotscli.Install(installOpts); err != nil {
		return err
	}

	// Continue with chart installation...
}
```

---

#### 3.2 Story sc-128060: Add missing functionality for the image pull secret template functions

**Purpose:** Enable HelmChart values to reference image pull secrets using template functions in both online and airgap installations.

**Implementation:**
- Current implementation of `CalculateRegistrySettings` and the corresponding template functions only supports airgap mode.
- Enhance `CalculateRegistrySettings` to support online mode by using the replicated proxy registry and the license ID as auth, but keep `HasLocalRegistry` as false.

```go
// api/internal/managers/linux/installation/config.go
func (m *installationManager) CalculateRegistrySettings(ctx context.Context, rc runtimeconfig.RuntimeConfig) (*types.RegistrySettings, error) {
   if m.airgapBundle == "" {

      authConfig := fmt.Sprintf(`{"auths":{"%s":{"username": "LICENSE_ID", "password": "%s"}}}`, replicatedProxyDomain, licenseID)
      imagePullSecretValue := base64.StdEncoding.EncodeToString([]byte(authConfig))

      return &types.RegistrySettings{
         HasLocalRegistry: false,
         ImagePullSecretName: "<app-slug>-registry",
         ImagePullSecretValue: imagePullSecretValue,
      }, nil
   }

   // Existing airgap mode implementation
}
```

### Iteration 4: Enhanced User Experience

---

#### 4.1 Story sc-128046: Update app install status endpoint to return list of charts

**Purpose:** Provide detailed installation progress information via API.

**Implementation:**
- Use same schema as infra components (api/types/infra.go pattern)
- Track installation progress in app install manager using Status type
- Return App struct with components array

```go
// api/types/app.go - New types (matching infra.go schema)
type App struct {
    Components []AppComponent `json:"components"`
    Logs       string         `json:"logs"`
    Status     Status         `json:"status"`
}

type AppComponent struct {
    Name   string `json:"name"`    // Chart name
    Status Status `json:"status"`  // Uses existing Status type with State/Description/LastUpdated
}

// api/controllers/app/install/status.go - New endpoint
func (c *AppInstallController) GetAppStatus(ctx context.Context) (*App, error) {
    status := c.appInstallManager.GetAppStatus()
    // Return App struct with component status using existing Status type
}
```

---

#### 4.2 Story sc-128047: Update app installation page to show charts with progress

**Purpose:** Display real-time chart installation progress in the UI.

**Implementation:**
- Follow same patterns as infra components display (LinuxInstallationPhase/KubernetesInstallationPhase)  
- Use existing StatusIndicator component for individual chart status
- Poll every 2 seconds using React Query (same as infra)

```typescript
// web/src/components/wizard/installation/phases/AppInstallationPhase.tsx
const AppInstallationPhase: React.FC = () => {
  const { data: appStatus } = useQuery({
    queryKey: ['app-install-status'],
    queryFn: () => fetch(`/api/${target}/install/app/status`).then(res => res.json()),
    refetchInterval: 2000  // Same as infra components
  });

  return (
    <div>
      {appStatus?.components?.map(component => (
        <StatusIndicator key={component.name} component={component} icon="server" />
      ))}
    </div>
  );
};
```

## Key Technical Decisions

### 1. V3 Binary Ownership
- **Decision:** V3 embedded-cluster binary takes full ownership of application deployment
- **Rationale:** Enables better control, reliability, and iterative improvement outside KOTS

### 2. KOTS Delegation Model
- **Decision:** KOTS delegates deployment to V3 binary but maintains metadata management
- **Rationale:** Preserves KOTS functionality for upgrades while transitioning deployment control

### 3. V3 API Isolation
- **Decision:** Only apply changes to V3 API installations
- **Rationale:** Zero risk to existing production installations
- **Benefit:** Can iterate and improve without backward compatibility concerns

### 4. Environment Variable Toggle
- **Decision:** Use EMBEDDED_CLUSTER_V3 environment variable for detection
- **Rationale:** Clear delegation mechanism, no feature flags required

## External Contracts

### API Endpoints (Iteration 4 Changes)
- **GET** `/linux/install/app/status` - Enhanced response structure with AppComponent array
- **GET** `/kubernetes/install/app/status` - Enhanced response structure with AppComponent array
- **POST** `/linux/install/app/install` - Same request structure (no changes)
- **POST** `/kubernetes/install/app/install` - Same request structure (no changes)

### New Response Types (Iteration 4)
```go
// Follows exact same schema as api/types/infra.go
type App struct {
    Components []AppComponent `json:"components"`
    Logs       string         `json:"logs"`
    Status     Status         `json:"status"`
}

type AppComponent struct {
    Name   string `json:"name"`
    Status Status `json:"status"`
}
```

### Environment Variables (Iteration 1)
- **IS_EMBEDDED_CLUSTER_V3** - New environment variable for KOTS/V3 coordination

### Preserved Contracts
- App install request structure unchanged
- Version record format unchanged
- All existing API endpoints maintain backward compatibility
- KOTS CLI integration preserved for metadata management

## Testing

### Iteration 1 Testing
**Unit Tests:**
- `install_test.go` - Helm client mock integration, basic chart installation flow
- V3 detection utility tests, environment variable handling
- KOTS skip deployment logic tests

**Integration Tests:**
- `appinstall_test.go` (Linux/Kubernetes) - V3 vs KOTS coordination scenarios
- Basic chart installation end-to-end flow

### Iteration 2 Testing
**Unit Tests:**
- `template_test.go` - HelmChart CR processing, values/optionalValues generation
- Controller orchestration tests between release and install managers
- Namespace, releaseName, exclude, weight, helmUpgradeFlags field handling
- Error scenarios for malformed HelmChart CRs

**Integration Tests:**
- Complex HelmChart scenarios with all supported fields
- Multi-chart installations with weight ordering
- Namespace creation and chart installation in custom namespaces
- Excluded chart scenarios (skip installation based on conditions)

### Iteration 3 Testing
**Unit Tests:**
- `airgap_test.go` - Image extraction and pushing logic
- `secrets_test.go` - Image pull secret creation and management
- Template function tests for registry-related functions
- Registry authentication scenarios

**Integration Tests:**
- Airgap installation end-to-end scenarios
- Image pull secret functionality across different namespaces
- Template function integration with actual chart values

### Iteration 4 Testing
**Unit Tests:**
- `status_test.go` - Progress tracking accuracy, concurrent access handling
- Chart status transitions and error state management
- API response structure validation

**Integration Tests:**
- Real-time progress reporting during chart installations
- UI integration testing for progress display components
- Status endpoint performance under load

**Frontend Tests:**
- Chart progress component testing with various status scenarios
- Real-time updates and error state handling
- User interaction and accessibility testing

### Compatibility Testing (All Iterations)
- Non-V3 installations continue to work unchanged (KOTS deployment)
- Existing V3 API functionality preserved
- KOTS CLI integration maintained for metadata management

## Backward Compatibility

### Backward Compatibility Maintained

**Installation Types:**
- **Non-V3 installations** continue using KOTS deployment unchanged (no risk to existing production)
- **All V3 installations** will have IS_EMBEDDED_CLUSTER_V3=true and use new direct Helm deployment
- **Clear separation:** V3 vs non-V3 installation types, no mixed modes

**KOTS Integration Preserved:**
- KOTS CLI integration maintained for metadata management
- Upgrade and lifecycle management continue working
- Version records and deployment history preserved

## Trade-offs

**Optimizing for:** Complete transition of application deployment ownership from KOTS to V3 embedded-cluster binary while maintaining end-to-end functionality

**Trade-offs made:**

### Architecture Trade-offs
- **Deployment Ownership Split:** V3 binary handles deployment, KOTS handles metadata management
  - *Rationale:* Enables gradual transition while preserving admin console functionality
  - *Mitigation:* Clear delegation model with IS_EMBEDDED_CLUSTER_V3 environment variable detection

- **Controller Orchestration Pattern:** Release manager processes HelmChart CRs, install manager handles installation
  - *Rationale:* Follows existing API architecture patterns (ExtractAppPreflightSpec model)
  - *Mitigation:* Proven pattern reduces integration risk

### Implementation Trade-offs
- **Sequential Chart Processing:** Charts installed one at a time vs parallel installation
  - *Rationale:* Simpler error handling, respects weight-based ordering, easier progress tracking
  - *Mitigation:* Can optimize for parallel processing in future iterations if needed

- **Flag Parsing Approach:** Use pflag library vs custom parsing for helmUpgradeFlags
  - *Rationale:* Robust handling of various flag formats, proven library used by helm/kubectl
  - *Mitigation:* Coordinate with data team to ensure vendor compatibility

## Alternative Solutions Considered

1. **Remove KOTS from V3 Entirely**
   - *Rejected:* Need admin console for upgrades and management
   - Would require significant changes beyond epic scope

2. **Gradual Feature Migration**
   - *Rejected:* Creates more complexity than clean delegation model
   - Would increase API surface area unnecessarily

## Research

### Prior Art in Codebase

**Helm Client Integration:**
- `pkg/helm/client.go` - Existing Helm client implementation patterns
- `api/internal/managers/linux/infra/util.go` - Helm client initialization and error handling patterns
- `pkg/addons/registry/install.go` - Registry integration patterns for Helm

**HelmChart Processing:**
- `api/pkg/template/engine.go` - Template processing engine (leverage for HelmChart CR templating)
- `api/pkg/template/registry.go` - Registry template functions (extend for image pull secrets)
- `api/internal/managers/app/release/template.go` - Existing `generateHelmValues` and `templateHelmChartCRs` functions

**Manager Architecture Patterns:**
- `api/internal/managers/app/release/manager.go` - `ExtractAppPreflightSpec` pattern (model for `ExtractInstallableHelmCharts`)
- `api/controllers/app/install/controller.go` - Controller orchestration between managers
- `api/internal/managers/linux/infra/` - Manager structure and dependency injection patterns

**Airgap and Registry:**
- `cmd/local-artifact-mirror/pull_images.go` - Image pulling and processing patterns
- `pkg/artifacts/registryauth.go` - Registry authentication handling
- `api/internal/managers/linux/installation/config.go` - `CalculateRegistrySettings` function (Line 240)

**Status and Progress Tracking:**
- `web/src/components/wizard/installation/phases/AppInstallationPhase.tsx` - Existing installation progress UI patterns
- `api/internal/handlers/linux/install.go` - Status endpoint patterns
- `api/types/app.go` - Existing app-related type definitions

**KOTS Integration:**
- `operator/controllers/installation_controller.go` - Existing operator patterns
- `pkg/configutils/kots.go` - KOTS configuration and utility patterns
- `cmd/installer/cli/install.go` - Environment variable handling patterns

### External References

**Helm Integration:**
- [Helm Go SDK Documentation](https://pkg.go.dev/helm.sh/helm/v3) - InstallOptions, client patterns
- [Helm CLI Source](https://github.com/helm/helm/blob/main/pkg/cmd/install.go#L187-L235) - Flag definitions for helmUpgradeFlags parsing
- [spf13/pflag Documentation](https://pkg.go.dev/github.com/spf13/pflag) - Flag parsing library

**Airgap Image Handling:**
- [KOTS Airgap Implementation](https://github.com/replicatedhq/kots/blob/main/pkg/image/airgap.go) - Image pushing patterns using containers/image/v5/copy
- [containers/image Documentation](https://pkg.go.dev/github.com/containers/image/v5) - Image manipulation library

**Architecture Patterns:**
- KOTS CLI Architecture - Metadata management delegation patterns
- Kubernetes Operator Patterns - Controller reconciliation and status management
