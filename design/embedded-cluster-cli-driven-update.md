# Embedded Cluster CLI-Driven Update Design

## Overview

This document outlines the implementation of **CLI-driven updates** to support the new architecture where customers will download a new binary to get the new release of software, compared to before where customers could check for versions in the UI to download and apply new versions.

The implementation will **modify the existing `./binary update` command** to perform CLI-driven binary updating when `ENABLE_V3=1` is set, instead of its current behavior of updating applications with airgap bundles.

## Architecture Overview

The CLI-driven update mechanism will leverage the existing embedded cluster download infrastructure while adding new components for version management, update detection, and safe binary replacement.

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
- **V3 mode**: CLI-driven binary updating behavior
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

**What Needs to be Added:**
- **Version comparison logic**: Compare current vs latest version using semantic versioning or cursor-based ordering depending on channel configuration
- **API integration**: Call new endpoint to check for latest version
- **Combined version info function**: Consolidate existing version data into single response

**Required Release Handling Strategy:**
Following the KOTS architecture pattern, the embedded cluster implementation should use a clear separation of responsibilities:

**API Responsibility:**
- **Return all available versions** with complete metadata including `required` flags
- **No filtering or blocking** - provide raw data to client
- **Include version ordering information** (channel sequence, release dates)

**Client Responsibility (Binary Update Logic):**
- **Determine update eligibility** based on required release rules
- **Enforce installation order** - prevent skipping required releases
- **Provide clear user feedback** when updates are blocked
- **Handle edge cases**:
  - User specifies `--version` but skips required releases
  - Multiple required releases between current and target version
  - Rollback scenarios with required releases

**User Experience Examples:**
```bash
# Normal update
./binary update --license license.yaml
→ "Update available: 1.6.0. Proceed? [y/N]"

# Blocked by required release
./binary update --version 1.6.0 --license license.yaml  
→ "Cannot update to 1.6.0. Required release 1.5.2 must be installed first."
→ "Run: ./binary update --version 1.5.2 --license license.yaml"

# Multiple required releases
./binary update --version 1.7.0 --license license.yaml
→ "Cannot update to 1.7.0. Required releases 1.5.2, 1.6.1 must be installed first."
→ "Suggested: ./binary update --license license.yaml (installs 1.5.2)"
```

**Semver vs Non-Semver Channel Handling:**
Following the KOTS pattern, the embedded cluster should handle these channel types differently in the client:

**API Behavior (Identical for Both):**
- Returns all releases with same metadata structure
- No semver-specific filtering or ordering by the API
- Provides `semverRequired` flag in channel metadata

**Client Logic Differences:**

**Non-Semver Channels (`semverRequired: false`):**
- **Cursor-based ordering**: Use channel sequence numbers for version comparison
- **Channel-scoped comparisons**: Only compare versions within the same channel
- **Required release scope**: Required releases only apply within current channel
- **Fallback ordering**: Use release dates or sequence numbers when other ordering fails

**Semver Channels (`semverRequired: true`):**
- **Semantic version ordering**: Parse and compare using semver rules (e.g., v1.2.3 > v1.2.1)
- **Cross-channel compatibility**: Can compare versions across different channels
- **Required release scope**: Required releases can block updates across ALL channels
- **Mixed version handling**: Handle channels with mix of semver and non-semver releases

**User Experience Differences:**
```bash
# Non-semver channel: sequence-based ordering
./binary update --license license.yaml
→ "Update available: sequence 156 (was 155). Proceed? [y/N]"

# Semver channel: semantic version ordering  
./binary update --license license.yaml
→ "Update available: v1.6.0 (from v1.5.2). Proceed? [y/N]"

# Cross-channel semver comparison
./binary update --version v1.7.0 --license license.yaml
→ "Cannot update to v1.7.0 from beta channel. Required release v1.6.5 from stable channel must be installed first."
```

**Key Point:** This leverages existing infrastructure rather than building new version management from scratch.

### 3. Update Discovery Mechanism

The binary needs to discover available updates by calling the replicated.app API.

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

### 4. Binary Download and Replacement (CLI-Driven Update Logic)

**Current State:** No existing binary update logic in embedded cluster binary.

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
