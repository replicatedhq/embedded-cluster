# Research: HelmChart Values and OptionalValues Support

## Executive Summary
This research document analyzes the existing `generateHelmValues` function and identifies the integration points needed to support values and optionalValues fields from HelmChart custom resources during V3 direct Helm installation.

## Current Implementation Analysis

### 1. Value Processing Function
Located at `api/internal/managers/app/release/template.go:generateHelmValues()`:

**Function Signature:**
```go
func generateHelmValues(templatedCR *kotsv1beta2.HelmChart) (map[string]any, error)
```

**Processing Logic:**
1. Starts with base values from `templatedCR.Spec.Values`
2. Iterates through `templatedCR.Spec.OptionalValues`
3. Evaluates "when" condition for each optionalValue
4. Merges values based on `RecursiveMerge` flag
5. Converts MappedChartValue to standard Go interfaces

### 2. HelmChart CR Templating
Located at `api/internal/managers/app/release/template.go:templateHelmChartCRs()`:

**Function Purpose:**
- Templates HelmChart CRs using config values
- Executes template engine with proxy spec support
- Returns templated HelmChart CR objects

**Key Operations:**
1. Parses YAML as template
2. Executes template with config values
3. Unmarshals back to HelmChart CR struct

### 3. Current Install Manager Gap

**Current Implementation (`api/internal/managers/app/install/install.go`):**
```go
func (m *appInstallManager) installHelmChart(ctx context.Context, chartArchive []byte, chartIndex int) error {
    // TODO: namespace should come from HelmChart custom resource
    // TODO: release name should come from HelmChart custom resource
    _, err = m.hcli.Install(ctx, helm.InstallOptions{
        ChartPath:   chartPath,
        Namespace:   constants.KotsadmNamespace,
        ReleaseName: ch.Metadata.Name,
        // Missing: Values field not populated
    })
}
```

**Identified Gaps:**
- No HelmChart CR processing
- No values extraction or processing
- No template engine integration
- Config values not available in install manager

## Integration Requirements

### 1. Data Flow
```
Config Values → Template Engine → HelmChart CR → generateHelmValues() → Helm Install
```

### 2. Required Components

**Template Engine:**
- Already exists in release manager
- Needs to be added to install manager
- Used for processing HelmChart CRs with config values

**Config Values:**
- Currently passed to KOTS CLI
- Need to be available in install manager
- Required for templating HelmChart CRs

**HelmChart CR Access:**
- Available in `m.releaseData.HelmChartCRs`
- Need correlation with chart archives by index
- Must handle missing CRs gracefully

### 3. Value Merging Logic

**Base Values:**
- Direct key-value pairs from `Spec.Values`
- Serve as foundation for configuration

**Optional Values:**
- Conditional based on "when" expression
- Two merge strategies:
  - Recursive merge using `kotsv1beta2.MergeHelmChartValues()`
  - Direct key replacement using `maps.Copy()`

## Implementation Considerations

### 1. Error Handling
- Template parsing failures
- Invalid "when" expressions
- Missing or malformed CRs
- Value conversion errors

### 2. Backward Compatibility
- Must handle releases without HelmChart CRs
- Default to no values if CRs unavailable
- Maintain existing behavior for non-V3 installs

### 3. Dependencies
The implementation depends on:
- `kotsv1beta2.HelmChart` type definitions
- `kotsv1beta2.MergeHelmChartValues()` function
- Template engine from `api/pkg/template`
- Existing `generateHelmValues` function

## Testing Requirements

### 1. Unit Test Coverage
- Template execution with various config values
- Value merging with different strategies
- Conditional optionalValues evaluation
- Error cases and edge conditions

### 2. Integration Test Scenarios
- Chart installation with complex values
- Multiple charts with different configurations
- Charts without HelmChart CRs
- Malformed or invalid CRs

## Risk Assessment

**Low Risk:**
- Reusing existing, tested functions
- Clear separation of concerns
- Graceful fallback behavior

**Medium Risk:**
- Template engine initialization complexity
- Config value threading through managers
- Index correlation between CRs and archives

## Recommendations

1. **Minimize Code Duplication:** Reuse existing `generateHelmValues` function rather than reimplementing
2. **Fail Gracefully:** Continue installation without values if CR processing fails
3. **Comprehensive Logging:** Add detailed logging for debugging value processing
4. **Incremental Testing:** Test each component independently before integration