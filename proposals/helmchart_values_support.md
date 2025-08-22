# Proposal: HelmChart Values and OptionalValues Field Support

**Status:** Proposed  
**Epic Proposal:** [V3 App Deployment Transition](./v3_app_deployment_transition.md)  
**Story:** [sc-128065](https://app.shortcut.com/replicated/story/128065)  
**Iteration:** 2 (HelmChart Resource Implementation)

## TL;DR

Add a new method to the app release manager to extract templated HelmChart CRs with processed values, then pass this data to the app install manager. This follows the established API architecture pattern where the controller orchestrates data flow between managers, similar to how `ExtractAppPreflightSpec` works.

## Scope

This proposal covers the integration of HelmChart custom resource values processing into the V3 installation flow:
- Add `ExtractHelmCharts` method to app release manager interface
- Process HelmChart CRs and generate values using existing `generateHelmValues` function
- Pass installable Helm charts to app install manager

**Out of scope for this story:**
- Other HelmChart CR fields (helmUpgradeFlags, weight, etc. - covered in other Iteration 2 stories)
- Template function enhancements (using existing functionality)
- Progress reporting (covered in Iteration 4)

See the [epic proposal](./v3_app_deployment_transition.md) for the complete implementation plan and architectural vision.

## Implementation

Following the established API architecture where **controllers orchestrate data flow between managers without cross-manager dependencies**:

**Core Changes:**
- `api/internal/managers/app/release/manager.go` - Add `ExtractInstallableHelmCharts` method to interface
- `api/internal/managers/app/release/template.go` - Implement extraction method using existing `generateHelmValues`
- `api/controllers/app/install/appinstall.go` - Call release manager to extract installable charts, then pass to install manager
- `api/internal/managers/app/install/install.go` - Update `Install` method to accept installable helm charts

**Technical Approach:**

1. **App Release Manager (Complete Data Processing):**
```go
type InstallableHelmChart struct {
    Archive []byte
    Values  map[string]any
    CR      *kotsv1beta2.HelmChart
}

type AppReleaseManager interface {
    ExtractAppPreflightSpec(ctx context.Context, configValues types.AppConfigValues, proxySpec *ecv1beta1.ProxySpec) (*troubleshootv1beta2.PreflightSpec, error)
    ExtractInstallableHelmCharts(ctx context.Context, configValues types.AppConfigValues, proxySpec *ecv1beta1.ProxySpec) ([]InstallableHelmChart, error) // New method
}
```

2. **Controller Orchestration (similar to preflight pattern):**
```go
// In app install controller
configValues, err := c.GetAppConfigValues(ctx)
installableCharts, err := c.appReleaseManager.ExtractInstallableHelmCharts(ctx, configValues, proxySpec)
err = c.appInstallManager.Install(ctx, installableCharts)
```

3. **App Install Manager (Installation Only):**
```go
// Update existing Install method signature
func (m *appInstallManager) Install(ctx context.Context, installableCharts []InstallableHelmChart) error
```

4. **Installation with Pre-Processed Values:**
```go
func (m *appInstallManager) installHelmChart(ctx context.Context, installableChart InstallableHelmChart) error {
    // Fallback to admin console namespace if namespace is not set
	namespace := installableChart.CR.GetNamespace()
	if namespace == "" {
		namespace = constants.KotsadmNamespace
	}

    // Values are already processed by release manager
    _, err = m.hcli.Install(ctx, helm.InstallOptions{
        ChartPath:   chartPath,
        Namespace:   namespace, // From HelmChart CR
        ReleaseName: installableChart.CR.GetReleaseName(), // From HelmChart CR
        Values:      installableChart.Values,  // Pre-processed from HelmChart CR
    })
}
```

**Key Technical Decisions:**
1. **Clean Separation of Concerns:** Release manager handles all data processing, install manager focuses on installation mechanics
2. **Single Source of Truth:** All chart data processing happens in the release manager
3. **Pre-Processed Data:** Values are processed once by the release manager, not during installation
4. **Complete Data Package:** Each InstallableHelmChart contains everything needed for installation

## Dependencies

- **Requires:** Direct Helm Chart Installation (sc-128045) - provides the basic installation infrastructure
- **Follows:** Same pattern as `ExtractAppPreflightSpec` - controller orchestration between release and install managers
- **Enables:** Other HelmChart CR field support stories in Iteration 2

## Testing

- Unit tests for `ExtractHelmCharts` method with various HelmChart CRs
- Unit tests for updated `Install` method with templated CRs
- Integration tests validating controller orchestration between managers
- Backward compatibility tests ensuring basic installation still works when CRs are nil
- Values precedence testing (base values vs optionalValues)

## Risk Assessment

**Low Risk Implementation:**
- Follows established API architecture patterns (`ExtractAppPreflightSpec`)
- Leverages existing, proven `generateHelmValues` function
- Clear separation of concerns between managers via controller orchestration
- Isolated to V3 API installations only
- Graceful fallback when HelmChart CRs are not present