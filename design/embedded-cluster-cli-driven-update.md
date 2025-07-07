# Embedded Cluster CLI-Driven Update Design

## Overview

This document outlines the implementation of a **CLI-first update management system** for embedded cluster binaries, representing a fundamental architectural shift from the traditional KOTS web UI-centric approach to direct command-line operations.

### Architectural Transformation

**Traditional KOTS Approach (Web UI-Centric):**
- Install kotsadm admin console (web UI) into the cluster
- Use port forwarding to access web interface on localhost:8800
- Manage applications through browser-based admin console
- CLI primarily used for bootstrapping the web UI

**New Embedded Cluster Approach (CLI-First):**
- Direct CLI commands for all operations without web UI dependency
- Self-updating binary that can manage both itself and applications
- All management happens through command-line interface
- No requirement for in-cluster web console

The implementation will **modify the existing `./binary update` command** to perform binary self-updating when `ENABLE_V3=1` is set, instead of its current behavior of updating applications with airgap bundles.

## Architecture Overview

The self-update mechanism will leverage the existing embedded cluster download infrastructure while adding new components for version management, update detection, and safe binary replacement.

```
Embedded Cluster Binary → Version Check → Download New Binary → Atomic Replacement
    ↓                        ↓              ↓                    ↓
[Current Binary]    [Version API]    [Download Endpoint]    [selfupdate]
```

## High-Level Changes Required

### 1. Command Line Interface Changes

#### Modified Update Command Behavior
- **Conditional behavior**: Check `ENABLE_V3` environment variable to determine update mode
- **V2 mode**: Current behavior (application airgap bundle updates)
- **V3 mode**: Binary self-updating behavior
- **Command signature (V3)**: `./binary update --license <license-file> [--version=<version>] [--force]`
- **License requirement**: The `--license` flag is **required** when `ENABLE_V3=1` for authentication with replicated.app
- **Validation**: Verify the binary can be updated (not installed via package manager)

#### Command Options (V3 Mode)
- `--license`: **Required** - License file for authentication with replicated.app
- `--version`: Optional parameter to specify target version (defaults to latest)
- `--force`: Skip confirmation prompts for automated scenarios

#### Backward Compatibility
- **Preserve V2 behavior**: When `ENABLE_V3` is not set or set to 0, maintain current airgap bundle update functionality

### 2. Version Management Infrastructure

**Current State:** Most version infrastructure already exists in the embedded cluster binary.

**What's Already Available:**
- Version information: `pkg/versions/versions.go` (EC version, k0s version)
- Embedded release data: App slug, channel, version label via `pkg/release/release.go`
- Version display: Existing `./binary version` command

#### Architecture Approach

**Dual Sequence Handling:**
The embedded cluster must handle multiple versioning schemes simultaneously, each serving different purposes:

- **Upstream cursors**: Channel sequence numbers from API (e.g., 1247, 1248, 1251, 1255)
  - Used for API synchronization and incremental updates
  - Can have gaps when releases are skipped or removed
  - Channel-specific and may restart across different channels
  - Essential for efficient API communication

- **Version labels**: User-facing identifiers (e.g., v1.2.0, v1.2.1)
  - What customers see and use in commands
  - May follow semantic versioning or custom labeling schemes
  - Used for `--version` flag and user communication
  - Channel-agnostic when semver is enabled

- **Embedded cluster versions**: Technical versions (e.g., 1.5.0+k8s-1.29)
  - Indicates the embedded cluster infrastructure version
  - Includes Kubernetes version and other technical details
  - Used for compatibility checks and technical validation
  - Independent of application versioning

**Example Mapping:**
```
API Release          User Experience      Technical Details
─────────────       ─────────────────     ─────────────────
Cursor: 1247    →   Version: v1.2.0   →   EC: 1.5.0+k8s-1.29
Cursor: 1248    →   Version: v1.2.1   →   EC: 1.5.0+k8s-1.29  
Cursor: 1251    →   Version: v1.3.0   →   EC: 1.6.0+k8s-1.30
```

This multi-layered approach allows the binary to handle API efficiency, user experience, and technical compatibility as separate concerns.

#### Version Comparison Logic

**Channel Configuration Driven:**
The comparison method depends on channel settings received from the API:

**Non-Semver Channels (`semverRequired: false`):**
- Use channel sequence numbers for version comparison
- Compare versions only within the same channel
- Required releases apply within current channel only
- Fallback to release dates when sequences unavailable

**Semver Channels (`semverRequired: true`):**
- Parse and compare using semantic versioning rules
- Required releases apply within current channel  
- Handle mixed semver/non-semver releases gracefully

#### Required Release Processing

**API Responsibility:**
- Return available versions (potentially cursor-based/incremental)
- Include `required` flags without filtering
- Provide version ordering information (sequences, dates)

**Client Responsibility:**
- Determine update eligibility based on required release rules
- Enforce installation order without skipping required releases
- Provide clear user feedback when updates are blocked
- Handle complex scenarios (multiple required releases, version validation)

#### User Experience Examples

**Normal Update:**
```bash
./binary update --license license.yaml
→ "Update available: v1.6.0 (from v1.5.2). Proceed? [y/N]"
```

**Update to Specific Version:**
```bash
./binary update --version v1.5.9 --license license.yaml
→ "Update available: v1.5.9 (from v1.5.2). Proceed? [y/N]"
```

**Blocked by Required Release:**
```bash
./binary update --version v1.6.0 --license license.yaml  
→ "Cannot update to v1.6.0. Required release v1.5.2 must be installed first."
→ "Run: ./binary update --version v1.5.2 --license license.yaml"
```

#### Implementation Considerations

**What Needs to be Added:**
- **Version comparison utilities**: Handle both semver and cursor-based ordering
- **API integration**: Call new endpoint to retrieve complete version lists
- **Required release validator**: Implement client-side logic without persistent state
- **Channel-aware logic**: Handle different comparison rules per channel type

**Key Point:** This leverages existing infrastructure while adapting proven version management patterns for a stateless, CLI-driven environment.

### 3. Update Discovery Mechanism

The system needs to discover available updates by calling the replicated.app API.

**New API Endpoint:** `GET /embedded/:appSlug/:channelSlug/versions`
- **Architecture**: Follow same proxy pattern as download endpoint (replicated-app → market-api)
- **Authentication**: Same license-based authentication as download endpoint
- **Authorization**: License ID in `Authorization` header
- **Validation**: Same app/channel access controls and feature flag checks

**API Response Format:**
```json
{
  "channel": {
    "id": "channel-123",
    "name": "Stable", 
    "slug": "stable",
    "semverRequired": true,
    "latestVersion": "1.2.3"
  },
  "versions": [
    {
      "versionLabel": "1.2.3",
      "channelSequence": 456,
      "ecVersion": "1.5.0+k8s-1.29",
      "releaseDate": "2024-01-15T10:30:00Z",
      "available": true,
      "airgapSupported": true,
      "required": false
    },
    {
      "versionLabel": "1.2.2", 
      "channelSequence": 455,
      "ecVersion": "1.4.0+k8s-1.28",
      "releaseDate": "2024-01-10T14:20:00Z",
      "available": true,
      "airgapSupported": true,
      "required": false
    }
  ]
}
```

**Key Implementation Requirements:**
- **Reuse existing infrastructure**: Same authentication, validation, and proxy patterns as download endpoint
- **Database optimization**: Efficient queries to avoid N+1 problems with large version lists
- **Error handling**: Consistent error responses and HTTP status codes with download endpoint
- **Performance**: Reasonable response times with appropriate caching headers

### 4. Binary Download and Replacement (TODO - Self-Update Logic)

**Current State:** No existing self-update logic in embedded cluster binary.

#### Download Process
Leverage existing embedded cluster download infrastructure:
- **Reuse download endpoint**: Use current `/embedded/:appSlug/:channelSlug/:versionLabel` pattern
- **Authentication**: Pass license ID from `--license` flag in Authorization header
- **Streaming download**: Handle large binaries efficiently without excessive memory usage
- **Progress indication**: Show download progress with percentage and transfer rates
- **Resume capability**: Support interrupted download resumption for reliability
- **Temporary storage**: Download to temp location before replacement

#### Atomic Replacement Strategy
Use proven `selfupdate` library approach (same as Replicated CLI):
- **Backup creation**: Always create backup of current binary before replacement
- **Atomic operation**: Use filesystem moves/renames for atomicity where possible
- **Permission preservation**: Maintain original executable permissions and ownership
- **Cross-platform support**: Handle platform-specific file replacement quirks
- **Verification post-replacement**: Confirm new binary works correctly

#### Error Handling and Rollback
Comprehensive error recovery mechanisms:
- **Download failures**: Network timeouts, authentication errors, disk space issues
- **Verification failures**: Checksum mismatches, corrupted downloads
- **Replacement failures**: Permission errors, file locks, disk full
- **Automatic rollback**: Restore backup if new binary fails basic validation
- **User guidance**: Clear error messages with suggested remediation steps
- **Cleanup**: Remove temporary files and failed downloads

#### Security Considerations
- **HTTPS enforcement**: All downloads over secure connections
- **License validation**: Verify license before attempting download
- **File permissions**: Ensure downloaded files have appropriate restrictions
- **Audit logging**: Log all update attempts for security monitoring

**Implementation Notes:**
- Mirror Replicated CLI's proven patterns for reliability
- Handle edge cases like running binary being updated
- Support both interactive and automated (scripted) usage
- Graceful degradation when binary update not possible

## Questions

### 1. State Management and API Cursor Support in Stateless Architecture

**Question:** How do we handle state-dependent operations like required release validation, cursor tracking, and deployment history in a stateless CLI environment, and should the versions API endpoint support cursor-based filtering or always return all versions?

**Context:** Embedded cluster's stateless architecture creates challenges for operations that typically require persistent state tracking. Without a local database or persistent storage, we need alternative approaches for:
- Tracking user's previous update path for required release validation
- Maintaining cursor positions for incremental API synchronization

### 2. Cross-Channel Semver Support

**Question:** Do we need to account for cross-channel semantic version comparisons, or can we simplify by restricting version comparisons to the same channel?

**Context:** The current design includes cross-channel semver functionality where:
- Semver-enabled channels can compare versions across different channels (e.g., v1.2.3 from stable > v1.2.1 from beta)
- Required releases can block updates across ALL channels when semver is enabled
- This adds significant complexity to the version comparison logic

### 3. Cursor-Based vs Semver-Only Version Management

**Question:** Should embedded cluster support cursor-based version management at all, or simplify by only supporting semantic versioning?

**Context:** The current design supports both cursor-based and semver version management like KOTS:
- **Cursor-based channels**: Use channel sequence numbers for ordering (1247, 1248, 1251, 1255)
- **Semver channels**: Use semantic version parsing and comparison (v1.2.0, v1.2.1, v1.3.0)

**Considerations:**
- **Complexity**: Supporting both systems significantly increases implementation complexity
- **Stateless challenges**: Cursor-based systems typically rely on persistent state for tracking
- **User experience**: Semver is more intuitive and widely understood
- **Compatibility**: Some existing channels may not use semantic versioning
