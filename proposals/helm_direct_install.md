# Proposal: Direct Helm Chart Installation for V3 API

**Status:** Proposed
**Epic Proposal:** [V3 App Deployment Transition](./v3_app_deployment_transition.md)
**Story:** [sc-128045](https://app.shortcut.com/replicated/story/128045)
**Iteration:** 1 (Foundation)

## TL;DR

Enhance the V3 API app installation manager to deploy Helm charts directly from releaseData.HelmChartArchives before calling KOTS CLI. This is the foundational story for transitioning application
deployment ownership from KOTS to the V3 embedded-cluster binary.

## Scope

This proposal covers only the basic Helm chart installation functionality needed for Iteration 1:
- Add Helm client to V3 API installation manager
- Install charts from releaseData.HelmChartArchives using default configuration
- Coordinate with KOTS CLI for metadata management

**Out of scope for this story:**
- HelmChart custom resource field support (covered in Iteration 2)
- Airgap image handling (covered in Iteration 3)
- Progress reporting UI (covered in Iteration 4)

See the [epic proposal](./v3_app_deployment_transition.md) for the complete implementation plan and architectural vision.

## Implementation

**Core Changes:**
- `api/internal/managers/app/install/manager.go` - Add Helm client field and constructor options
- `api/internal/managers/app/install/install.go` - Add `installHelmCharts()` method before KOTS CLI call
- `api/internal/managers/app/install/util.go` - Add Helm client setup utilities

**Key Technical Decisions:**
1. **Basic Chart Installation Only:** Use chart name as release name, install to kotsadm namespace
2. **Sequential Processing:** Install charts one at a time for reliability
3. **No Rollback Logic:** Fail fast on errors, consistent with existing patterns

## Testing

- Unit tests with Helm client mocks in `install_test.go`
- Integration tests using unified releaseData structure
- Compatibility verification that non-V3 installations remain unchanged

## Dependencies

- Requires KOTS skip deployment changes (sc-128049) to prevent conflicts
- Foundation for HelmChart custom resource support in Iteration 2

The key changes:
1. Reduced scope - Focus only on the basic Helm installation for this specific story
2. Clear references - Point to the epic proposal for the bigger picture
3. Iteration context - Explicitly state this is Iteration 1 foundation work
4. Dependencies - Clear about what this depends on and enables
5. Removed duplicate content - Architecture, risk assessment, etc. are in the epic proposal