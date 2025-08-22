# Proposal: Transition Application Deployment to V3 Embedded-Cluster

**Status:** Proposed
**Epic:** [Installer Experience v2 - Milestone 7](https://app.shortcut.com/replicated/epic/126565)
**Related Stories:** sc-128045, sc-128049

## TL;DR

Transition application deployment from KOTS to the V3 embedded-cluster binary to deliver Helm charts directly through our installer instead of KOTS. This enables better control, reliability, and
user experience while maintaining KOTS functionality for upgrades and management, delivered through iterative milestones.

## The Problem

This proposal addresses the larger architectural goal of controlling application lifecycle management outside the cluster through the embedded-cluster binary, while ensuring KOTS continues to
function end-to-end during the transition.

**Long-term vision:** Move entire application deployment and lifecycle management from in-cluster KOTS to the external embedded-cluster binary for better control, reliability, and user experience.

**Current iteration goal:** Enable the V3 embedded-cluster binary to handle initial application deployment with full HelmChart resource support while allowing KOTS to seamlessly take over lifecycle
management post-install.

**Current technical problem:** The V3 API installation manager delegates entirely to KOTS CLI for application deployment, preventing direct control over Helm chart installation and HelmChart custom
resource configuration. Both KOTS and the V3 installer attempt to deploy applications during initial install, creating resource conflicts and unclear ownership.

## Prototype / Design

The solution transitions application deployment from KOTS to the V3 embedded-cluster binary:

┌─────────────────┐
│  V3 Install API │
└────────┬────────┘
         │
         ▼
┌─────────────────┐     ┌──────────────────┐
│ Setup Helm      │────▶│ Install Charts   │
│ Client          │     │ (V3 Binary)      │
└─────────────────┘     └──────────┬───────┘
                                    │
                                    ▼
                        ┌──────────────────┐
                        │ Call KOTS CLI    │
                        │ (Skip Deploy)    │
                        └──────────────────┘

**Flow Details:**
1. V3 binary takes ownership of application deployment
2. Setup Helm client following infra manager patterns
3. Install charts directly from releaseData.HelmChartArchives with full HelmChart custom resource support
4. Call KOTS CLI with SkipDeploy: true for metadata management only
5. Fail fast on errors without complex rollback logic

**KOTS Coordination:** KOTS skips application deployment during V3 initial installs when `EMBEDDED_CLUSTER_V3=true` environment variable is set, preventing resource conflicts while maintaining
version records and metadata.

## Implementation Plan

### Iteration 1: Core Deployment Transition (Stories sc-128045, sc-128049)
- **V3 Binary Takes Ownership**
   - Add Helm client to appInstallManager with constructor options
   - Update install() method to deploy charts before KOTS CLI
   - Implement chart installation from releaseData.HelmChartArchives

- **KOTS Delegates to V3**
   - Modify KOTS DeployApp() to skip deployment for V3 EC initial installs
   - Maintain version records and metadata creation
   - Add V3 detection via EMBEDDED_CLUSTER_V3 environment variable

### Iteration 2: Full HelmChart Resource Implementation
- **HelmChart Fields**
   - Support namespace field configuration (sc-128055)
   - Support exclude field for conditional chart deployment (sc-128056)
   - Support chart weight ordering for installation sequence (sc-128057)
   - Support helmUpgradeFlags for deployment customization (sc-128058)
   - Support releaseName field customization (sc-128062)
   - Support values and optionalValues field configuration (sc-128065)

### Iteration 3: Complete Airgap Transition
- **V3 Binary Handles Airgap**
   - Push airgap images from bundle to EC registry without KOTS (sc-128061)
   - Create image pull secrets for applications (sc-128059)
   - Add support for image pull secret template functions (sc-128060)

### Iteration 4: Enhanced User Experience
- **V3 Binary Progress Reporting**
   - Update app install status endpoint to return chart installation progress (sc-128046)
   - Update installation page UI to show charts being installed with progress (sc-128047)

## Key Technical Decisions

1. **V3 Binary Ownership**
   - **Decision:** V3 embedded-cluster binary takes full ownership of application deployment
   - **Rationale:** Enables better control, reliability, and iterative improvement outside KOTS

2. **KOTS Delegation Model**
   - **Decision:** KOTS delegates deployment to V3 binary but maintains metadata management
   - **Rationale:** Preserves KOTS functionality for upgrades while transitioning deployment control

3. **V3 API Isolation**
   - **Decision:** Only apply changes to V3 API installations
   - **Rationale:** Zero risk to existing production installations
   - **Benefit:** Can iterate and improve without backward compatibility concerns

4. **Environment Variable Toggle**
   - **Decision:** Use EMBEDDED_CLUSTER_V3 environment variable for detection
   - **Rationale:** Clear delegation mechanism, no feature flags required

## Files to Modify

**Core Changes:**
- `api/internal/managers/app/install/manager.go` - Add Helm client fields and constructor options
- `api/internal/managers/app/install/install.go` - Update install() method to call Helm before KOTS CLI
- `api/internal/managers/app/install/util.go` - Move utility functions following code organization patterns
- `pkg/operator/operator.go` - Modify DeployApp() function to skip deployment for V3 EC initial installs
- `pkg/util/util.go` - Add V3 detection utilities

**Test Updates:**
- `api/internal/managers/app/install/install_test.go` - Enhance unit tests with Helm client mocks
- `api/integration/kubernetes/install/appinstall_test.go` - Update integration tests with unified releaseData
- `api/integration/linux/install/appinstall_test.go` - Update integration tests with unified releaseData
- `pkg/operator/operator_test.go` - Add unit tests for V3 detection logic

## External Contracts

No changes to external APIs or events:
- Same request/response structure for `/install/app` endpoint
- Version record format unchanged
- API contracts preserved

## Risk Assessment

**Technical Risks:**
- V3 binary deployment integration issues (Low probability, Medium impact)
- Chart installation failures (Medium probability, Medium impact)
- KOTS delegation coordination issues (Low probability, High impact)

**Business Risks:**
- V3 deployment regressions (Low probability, High impact)
- Increased complexity during transition (Low probability, Medium impact)
- Support burden during dual-mode operation (Low probability, Low impact)

## Testing

**Unit Tests:**
- Enhance TestAppInstallManager_Install with Helm client mocks
- Test coverage for V3 binary chart installation and error handling
- V3 detection logic and initial install vs upgrade scenarios

**Integration Tests:**
- Modify TestPostInstallApp tests to validate V3 binary deployment
- Validate V3 API installation flow with direct Helm charts
- Ensure HelmChart custom resource fields are respected by V3 binary

**Compatibility Tests:**
- Non-V3 installations continue to work unchanged (KOTS deployment)
- Existing V3 API functionality preserved
- KOTS CLI integration maintained for metadata management

## Backward Compatibility

Fully backward compatible:
- Only affects V3 API installations when EMBEDDED_CLUSTER_V3=true
- All existing installation types unchanged (continue using KOTS deployment)
- KOTS CLI integration preserved for metadata management
- API contracts preserved

## Trade-offs

**Optimizing for:** Complete transition of application deployment ownership from KOTS to V3 embedded-cluster binary while maintaining end-to-end functionality

**Trade-offs made:**
- **Deployment Ownership Split:** V3 binary handles deployment, KOTS handles metadata
   - *Mitigation:* Clear delegation model with environment variable detection
- **Testing surface:** Need to test both V3 binary deployment and KOTS delegation
   - *Mitigation:* Comprehensive test coverage for both integration points
- **Sequential processing:** Charts installed one at a time vs parallel installation
   - *Mitigation:* Simpler, more reliable approach; can optimize later if needed
- **Transition complexity:** Dual-mode operation during migration period
   - *Mitigation:* Clear boundaries and comprehensive testing

## Alternative Solutions Considered

1. **Remove KOTS from V3 Entirely**
   - *Rejected:* Need admin console for upgrades and management
   - Would require significant changes beyond epic scope

2. **Gradual Feature Migration**
   - *Rejected:* Creates more complexity than clean delegation model
   - Would increase API surface area unnecessarily

3. **New Installation Interface**
   - *Rejected:* User noted no new methods needed, just update existing Install method
   - Clean delegation model achieves the same goal

## Research

**Prior Art in Codebase:**
- Helm Client Setup: `api/internal/managers/linux/infra/util.go` patterns for Helm client initialization
- Component Logging: `api/internal/managers/linux/infra/util.go` patterns for structured logging
- Release Data: Using existing `releaseData.HelmChartArchives` data structure

**External References:**
- Helm Go SDK Documentation
- KOTS CLI Architecture
- Kubernetes operator deployment strategies