# Proposal: Direct Helm Chart Installation for V3 API

**Status**: Proposed  
**Related**: Works in conjunction with KOTS skip deployment proposal (sc-128049)

## TL;DR
Enhance the V3 API app installation manager to deploy Helm charts directly from `releaseData.HelmChartArchives` before calling KOTS CLI, enabling the V3 installer to handle application deployment while maintaining KOTS functionality for upgrades and management.

## The Problem

This proposal is an iterative step toward the larger architectural goal of controlling application lifecycle management outside the cluster through the embedded-cluster binary, while ensuring KOTS continues to function end-to-end during the transition.

**Long-term vision**: Move application deployment and lifecycle management from in-cluster KOTS to the external embedded-cluster binary for better control, reliability, and user experience.

**Current iteration goal**: Enable the V3 embedded-cluster API to handle initial application deployment while allowing KOTS to seamlessly take over lifecycle management post-install. This iteration uses basic default configuration for all charts and does not yet integrate with HelmChart custom resources for chart-specific deployment configuration. This approach:

- **Maintains KOTS functionality**: Admin console, upgrades, configuration management remain intact
- **Enables iterative development**: Allows V3 API development to proceed without breaking existing KOTS workflows  
- **Preserves upgrade path**: KOTS can manage subsequent deployments after initial install
- **Reduces scope**: Smaller, manageable change that keeps the system working end-to-end
- **Establishes foundation**: Core deployment mechanism can be enhanced with HelmChart custom resource integration in future iterations

**Current technical problem**: The V3 API installation manager delegates entirely to KOTS CLI for application deployment, preventing direct control over Helm chart installation. We need the V3 API to deploy charts directly using existing `releaseData.HelmChartArchives` data while maintaining coordination with KOTS CLI.

**Evidence**: V3 API development requires this separation to proceed with external lifecycle management while keeping KOTS operational for existing features. The complementary KOTS skip deployment proposal (sc-128049) will prevent double deployment conflicts.

## Prototype / Design

The solution introduces direct Helm chart deployment in the V3 API installation flow:

```
┌─────────────────┐
│  V3 Install API │
└────────┬────────┘
         │
         ▼
┌─────────────────┐     ┌──────────────────┐
│ Setup Helm      │────▶│ Install Charts   │
│ Client          │     │ from ReleaseData │
└─────────────────┘     └──────────┬───────┘
                                   │
                                   ▼
                        ┌──────────────────┐
                        │ Call KOTS CLI    │
                        │ SkipPreflights   │
                        └──────────────────┘
```

**Flow Details**:
1. **Setup Helm client** following infra manager patterns
2. **Install charts** directly from `releaseData.HelmChartArchives` 
3. **Call KOTS CLI** with `SkipPreflights: true` for metadata management
4. **Fail fast** on errors without complex rollback logic

**Coordination with KOTS**: The complementary KOTS skip deployment proposal (sc-128049) ensures KOTS doesn't attempt chart deployment when called by V3 API, preventing resource conflicts.

## Key Technical Decisions

### 1. No Complex Rollback Logic
- **Decision**: Return errors immediately on Helm failures
- **Rationale**: Simplicity, reliability, aligns with existing error handling patterns
- **Risk**: Partial installations possible, but consistent with current behavior

### 2. V3 API Isolation
- **Decision**: Only apply changes to V3 API installations  
- **Rationale**: Zero risk to existing production installations
- **Benefit**: Can iterate and improve without backward compatibility concerns

### 3. Leverage Existing Infrastructure
- **Decision**: Reuse infra manager patterns for Helm client setup
- **Rationale**: Proven patterns, consistent codebase, reduced development time
- **Benefit**: Familiar debugging and monitoring approaches

## Implementation Plan

### Files to Modify

**Core Changes**:
- `api/internal/managers/app/install/manager.go` - Add Helm client fields and constructor options
- `api/internal/managers/app/install/install.go` - Update install() method to call Helm before KOTS CLI  
- `api/internal/managers/app/install/util.go` - Move utility functions following code organization patterns

**Test Updates**:
- `api/internal/managers/app/install/install_test.go` - Enhance unit tests with Helm client mocks
- `api/integration/kubernetes/install/appinstall_test.go` - Update integration tests with unified releaseData
- `api/integration/linux/install/appinstall_test.go` - Update integration tests with unified releaseData

### Toggle Strategy

**V3 API Only**: Change only applies to V3 API installations
- No feature flags or environment variables required
- Clear architectural boundary with existing installations
- Zero impact on current production deployments

### External Contracts

No changes to external APIs or events:
- Same request/response structure for `/install/app` endpoint
- Version record format unchanged  
- API contracts preserved

## Risk Assessment

### Technical Risks
| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| Helm client integration issues | Low | Medium | Leverage proven infra manager patterns |
| Chart installation failures | Medium | Medium | Fail fast, clear error messages |
| KOTS CLI coordination issues | Low | High | Maintain existing KOTS CLI integration |

### Business Risks
| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| V3 API deployment regressions | Low | High | Comprehensive testing, V3-only scope |
| Increased complexity | Low | Medium | Follow existing patterns, minimal changes |
| Support burden | Low | Low | Component-specific logging, familiar patterns |

## Testing

### Unit Tests
- Enhance `TestAppInstallManager_Install` test cases to include Helm client mocks
- Add test coverage for Helm chart installation and error handling  
- Update test data to use unified `releaseData` structure with `HelmChartArchives`
- Verify both successful installation and error scenarios

### Integration Tests  
- Modify `TestPostInstallApp` tests to use `releaseData` with Helm chart archives
- Ensure all test cases use consistent release data structure
- Validate V3 API installation flow with direct Helm charts

### Compatibility Tests
- Non-V3 installations will continue to work unchanged
- Existing V3 API functionality will be preserved
- KOTS CLI integration will be maintained

## Backward Compatibility

**Fully backward compatible**:
- Only affects V3 API installations 
- All existing installation types unchanged
- KOTS CLI integration preserved for metadata management
- API contracts preserved

**Forward compatibility**:
- V3 installations can upgrade to future versions normally
- Foundation for additional V3 API enhancements
- Maintains KOTS functionality for long-term management

## Trade-offs

**Optimizing for**: Clean separation of concerns between KOTS and V3 installer while maintaining end-to-end functionality

**Trade-offs made**:

1. **Complexity**: Adding Helm deployment logic to V3 API installation flow
   - **Mitigation**: Will leverage proven infra manager patterns, well-isolated changes
   
2. **Testing surface**: Need to test both V3 API Helm deployment and KOTS CLI coordination  
   - **Mitigation**: Will implement comprehensive test coverage for both integration points
   
3. **Sequential processing**: Charts will be installed one at a time vs parallel installation
   - **Mitigation**: Simpler, more reliable approach; can optimize later if needed
   
4. **No rollback logic**: Helm failures will result in partial installations
   - **Mitigation**: Fail fast approach aligns with existing error handling patterns

## Alternative Solutions Considered

### 1. Remove KOTS from V3 Entirely
- **Rejected**: Need admin console for upgrades and management for now
- Would require significant changes and is out of scope for this iteration

### 2. New Installation Interface
- **Rejected**: User noted no new methods needed, just update existing Install method
- Would increase API surface area unnecessarily  

### 3. Parallel Chart Installation  
- **Deferred**: Sequential approach simpler and more reliable initially
- Can optimize for parallel installation in future iterations

## Checkpoints (PR Plan)

### Single PR: Complete Implementation
**Scope**: 
- Add Helm client to appInstallManager struct with constructor options
- Update install() method to call Helm installation before KOTS CLI
- Move utility functions to util.go following code organization patterns  
- Enhance unit tests with Helm client mocks and error scenarios
- Update integration tests to use unified releaseData structure
- Document component logging approach

The change is isolated enough that breaking it into multiple PRs would add unnecessary overhead without improving reviewability.

## Research

### Prior Art in Codebase
- **Helm Client Setup**: `api/internal/managers/linux/infra/util.go` patterns for Helm client initialization
- **Component Logging**: `api/internal/managers/linux/infra/util.go` patterns for structured logging
- **Release Data**: Using existing `releaseData.HelmChartArchives` data structure

### External References  
- [Helm Go SDK Documentation](https://helm.sh/docs/topics/advanced/#go-sdk)
- [KOTS CLI Architecture](https://docs.replicated.com/reference/kots-cli-install)